package privhelper

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/rawprobe"
	"github.com/soyunomas/storverity/internal/safety"
)

var (
	ErrUnauthorized = errors.New("authorization denied")
	ErrBusy         = errors.New("another privileged raw probe is already running")
	ErrProtected    = errors.New("selected device is not eligible for raw probing")
	ErrIdentity     = errors.New("selected device identity changed")
	ErrInvalid      = errors.New("invalid privileged raw probe request")
)

type Authorizer interface {
	Authorize(context.Context, string, string, map[string]string) error
}

type DeviceSource interface {
	List(context.Context) ([]device.Device, error)
}

type ProbeEngine interface {
	Run(context.Context, rawprobe.Media, rawprobe.Config, func(rawprobe.Progress)) (rawprobe.Report, error)
}

type MediaOpener interface {
	Open(path string, expectedMajorMinor string) (rawprobe.Media, error)
}

type MediaOpenFunc func(path string, expectedMajorMinor string) (rawprobe.Media, error)

func (f MediaOpenFunc) Open(path string, expectedMajorMinor string) (rawprobe.Media, error) {
	return f(path, expectedMajorMinor)
}

type activeRun struct {
	caller    string
	sessionID string
	cancel    context.CancelFunc
}

type Server struct {
	devices    DeviceSource
	engine     ProbeEngine
	opener     MediaOpener
	authorizer Authorizer
	seed       func() ([32]byte, error)

	mu     sync.Mutex
	active *activeRun
}

func NewServer(devices DeviceSource, engine ProbeEngine, opener MediaOpener, authorizer Authorizer) *Server {
	return &Server{
		devices: devices, engine: engine, opener: opener, authorizer: authorizer,
		seed: randomSeed,
	}
}

func NewLinuxServer(authorizer Authorizer) *Server {
	return NewServer(
		device.NewScanner(),
		rawprobe.New(),
		MediaOpenFunc(rawprobe.OpenLinuxBlockDevice),
		authorizer,
	)
}

func (s *Server) RunRawProbe(ctx context.Context, caller string, req RunRequest, emit func(rawprobe.Progress)) RunResponse {
	if err := s.validateRequest(req); err != nil {
		return responseError(ErrorInvalid, err)
	}
	if s == nil || s.devices == nil || s.engine == nil || s.opener == nil || s.authorizer == nil || s.seed == nil {
		return responseError(ErrorInternal, errors.New("privileged helper is not configured"))
	}
	samples, blockBytes, err := normalizedGeometry(req)
	if err != nil {
		return responseError(ErrorInvalid, err)
	}
	if s.isBusy() {
		return responseError(ErrorBusy, ErrBusy)
	}
	if err := s.authorizer.Authorize(ctx, caller, PolkitAction, map[string]string{
		"device-id": req.DeviceID,
		"operation": "raw-capacity-probe",
	}); err != nil {
		return responseError(ErrorUnauthorized, fmt.Errorf("authorize raw probe: %w", err))
	}

	runCtx, finish, err := s.beginRun(ctx, caller, req.SessionID)
	if err != nil {
		return responseError(ErrorBusy, err)
	}
	defer finish()

	selected, err := s.resolveEligible(runCtx, req.DeviceID)
	if err != nil {
		return classifyPreOpenError(err)
	}
	if device.Fingerprint(selected) != req.ExpectedFingerprint {
		return responseError(ErrorIdentity, ErrIdentity)
	}

	media, err := s.opener.Open(selected.Path, selected.MajorMinor)
	if err != nil {
		return responseError(ErrorOpen, fmt.Errorf("open confirmed raw target: %w", err))
	}
	defer media.Close()

	// Re-discover after the privileged descriptor has been opened and before
	// the first write. This mirrors the desktop safety gate but makes the root
	// helper authoritative against path replacement, re-insertion, or a mount
	// racing the polkit prompt/open sequence.
	opened, err := s.resolveEligible(runCtx, req.DeviceID)
	if err != nil {
		return classifyPreOpenError(err)
	}
	if device.Fingerprint(opened) != req.ExpectedFingerprint || opened.Path != selected.Path || opened.MajorMinor != selected.MajorMinor {
		return responseError(ErrorIdentity, ErrIdentity)
	}

	seed, err := s.seed()
	if err != nil {
		return responseError(ErrorInternal, fmt.Errorf("create raw probe seed: %w", err))
	}

	report, runErr := s.engine.Run(runCtx, media, rawprobe.Config{
		CapacityBytes: opened.SizeBytes,
		Samples:       samples,
		BlockBytes:    blockBytes,
		Seed:          seed,
	}, emit)
	if runErr == nil {
		return RunResponse{Report: report}
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runCtx.Err(), context.Canceled) {
		return RunResponse{Report: report, Error: &RunError{Code: ErrorCancelled, Message: context.Canceled.Error()}}
	}
	return RunResponse{Report: report, Error: &RunError{Code: ErrorProbe, Message: fmt.Sprintf("raw capacity probe: %v", runErr)}}
}

func (s *Server) Cancel(caller, sessionID string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if active == nil || active.caller != caller || active.sessionID != sessionID {
		return false
	}
	active.cancel()
	return true
}

func (s *Server) CancelCaller(caller string) bool {
	if s == nil || strings.TrimSpace(caller) == "" {
		return false
	}
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if active == nil || active.caller != caller {
		return false
	}
	active.cancel()
	return true
}

func (s *Server) validateRequest(req RunRequest) error {
	if s == nil {
		return ErrInvalid
	}
	if !validToken(req.SessionID) || strings.TrimSpace(req.DeviceID) == "" || strings.TrimSpace(req.ExpectedFingerprint) == "" {
		return ErrInvalid
	}
	return nil
}

func normalizedGeometry(req RunRequest) (int, uint64, error) {
	samples := req.Samples
	if samples < rawprobe.DefaultSamples {
		samples = rawprobe.DefaultSamples
	}
	blockBytes := req.BlockBytes
	if blockBytes == 0 {
		blockBytes = rawprobe.DefaultBlockBytes
	}
	if samples > rawprobe.MaxSamples || blockBytes < rawprobe.MinBlockBytes || blockBytes > rawprobe.MaxBlockBytes || blockBytes&(blockBytes-1) != 0 {
		return 0, 0, ErrInvalid
	}
	return samples, blockBytes, nil
}

func validToken(value string) bool {
	if len(value) < 16 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func (s *Server) isBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active != nil
}

func (s *Server) beginRun(parent context.Context, caller, sessionID string) (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return nil, nil, ErrBusy
	}
	runCtx, cancel := context.WithCancel(parent)
	s.active = &activeRun{caller: caller, sessionID: sessionID, cancel: cancel}
	finish := func() {
		cancel()
		s.mu.Lock()
		if s.active != nil && s.active.caller == caller && s.active.sessionID == sessionID {
			s.active = nil
		}
		s.mu.Unlock()
	}
	return runCtx, finish, nil
}

func (s *Server) resolveEligible(ctx context.Context, selectedID string) (device.Device, error) {
	items, err := s.devices.List(ctx)
	if err != nil {
		return device.Device{}, fmt.Errorf("refresh devices: %w", err)
	}
	for _, d := range items {
		if device.ID(d) != selectedID {
			continue
		}
		if decision := safety.EvaluateRawTest(d); !decision.Allowed || strings.TrimSpace(d.MajorMinor) == "" {
			return device.Device{}, ErrProtected
		}
		return d, nil
	}
	return device.Device{}, ErrIdentity
}

func classifyPreOpenError(err error) RunResponse {
	switch {
	case errors.Is(err, ErrProtected):
		return responseError(ErrorProtected, err)
	case errors.Is(err, ErrIdentity):
		return responseError(ErrorIdentity, err)
	case errors.Is(err, context.Canceled):
		return responseError(ErrorCancelled, context.Canceled)
	default:
		return responseError(ErrorInternal, err)
	}
}

func responseError(code ErrorCode, err error) RunResponse {
	message := "unknown error"
	if err != nil {
		message = err.Error()
	}
	return RunResponse{Error: &RunError{Code: code, Message: message}}
}

func randomSeed() ([32]byte, error) {
	var seed [32]byte
	_, err := rand.Read(seed[:])
	return seed, err
}

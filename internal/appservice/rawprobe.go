package appservice

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/rawprobe"
	"github.com/soyunomas/storverity/internal/safety"
)

var (
	ErrRawProbeProtected    = errors.New("selected device is not eligible for raw probing")
	ErrRawProbeChallenge    = errors.New("raw probe confirmation challenge is invalid or expired")
	ErrRawProbeConfirmation = errors.New("raw probe confirmation text does not match")
	ErrRawProbeIdentity     = errors.New("selected device identity changed after confirmation")
)

const defaultRawChallengeTTL = 90 * time.Second

type RawProbeEngine interface {
	Run(context.Context, rawprobe.Media, rawprobe.Config, func(rawprobe.Progress)) (rawprobe.Report, error)
}

type RawMediaOpener interface {
	Open(path string, expectedMajorMinor string) (rawprobe.Media, error)
}

type RawMediaOpenFunc func(path string, expectedMajorMinor string) (rawprobe.Media, error)

func (f RawMediaOpenFunc) Open(path string, expectedMajorMinor string) (rawprobe.Media, error) {
	return f(path, expectedMajorMinor)
}

type RawProbeChallenge struct {
	Token            string `json:"token"`
	DeviceID         string `json:"deviceId"`
	Path             string `json:"path"`
	DisplayName      string `json:"displayName"`
	CapacityBytes    uint64 `json:"capacityBytes"`
	ConfirmationText string `json:"confirmationText"`
	ExpiresAtUnix    int64  `json:"expiresAtUnix"`
}

type RawProbeRequest struct {
	DeviceID       string `json:"deviceId"`
	ChallengeToken string `json:"challengeToken"`
	Confirmation   string `json:"confirmation"`
	Samples        int    `json:"samples"`
	BlockBytes     uint64 `json:"blockBytes"`
}

type RawProbeProgress = rawprobe.Progress

type rawChallengeRecord struct {
	deviceID     string
	fingerprint  string
	confirmation string
	expires      time.Time
}

type RawProbeController struct {
	devices DeviceSource
	engine  RawProbeEngine
	opener  RawMediaOpener
	seed    func() ([32]byte, error)
	token   func() (string, error)
	now     func() time.Time
	ttl     time.Duration

	mu         sync.Mutex
	challenges map[string]rawChallengeRecord
}

func NewRawProbeController(devices DeviceSource, engine RawProbeEngine, opener RawMediaOpener) *RawProbeController {
	return &RawProbeController{
		devices: devices, engine: engine, opener: opener, seed: randomSeed,
		token: randomRawToken, now: time.Now, ttl: defaultRawChallengeTTL,
		challenges: make(map[string]rawChallengeRecord),
	}
}

func (c *RawProbeController) Prepare(ctx context.Context, selectedID string) (RawProbeChallenge, error) {
	if c == nil || c.devices == nil || c.engine == nil || c.opener == nil {
		return RawProbeChallenge{}, errors.New("raw probe controller is not configured")
	}
	if strings.TrimSpace(selectedID) == "" {
		return RawProbeChallenge{}, errors.New("device id is required")
	}
	d, err := c.refreshDevice(ctx, selectedID)
	if err != nil {
		return RawProbeChallenge{}, err
	}
	if decision := safety.EvaluateRawTest(d); !decision.Allowed || strings.TrimSpace(d.MajorMinor) == "" {
		return RawProbeChallenge{}, ErrRawProbeProtected
	}
	token, err := c.token()
	if err != nil {
		return RawProbeChallenge{}, fmt.Errorf("create raw probe challenge: %w", err)
	}
	now := c.now()
	ttl := c.ttl
	if ttl <= 0 {
		ttl = defaultRawChallengeTTL
	}
	expires := now.Add(ttl)
	confirmation := "DESTROY DATA ON " + d.Path
	record := rawChallengeRecord{
		deviceID: selectedID, fingerprint: rawDeviceFingerprint(d),
		confirmation: confirmation, expires: expires,
	}
	c.mu.Lock()
	for key, existing := range c.challenges {
		if !existing.expires.After(now) {
			delete(c.challenges, key)
		}
	}
	c.challenges[token] = record
	c.mu.Unlock()
	return RawProbeChallenge{
		Token: token, DeviceID: selectedID, Path: d.Path, DisplayName: displayName(d),
		CapacityBytes: d.SizeBytes, ConfirmationText: confirmation, ExpiresAtUnix: expires.Unix(),
	}, nil
}

func (c *RawProbeController) Run(ctx context.Context, req RawProbeRequest, emit func(RawProbeProgress)) (rawprobe.Report, error) {
	if c == nil || c.devices == nil || c.engine == nil || c.opener == nil {
		return rawprobe.Report{}, errors.New("raw probe controller is not configured")
	}
	record, err := c.takeChallenge(req)
	if err != nil {
		return rawprobe.Report{}, err
	}

	// First refresh: the selection must still identify an eligible target before
	// we ask the OS to open anything writable.
	d, err := c.refreshDevice(ctx, req.DeviceID)
	if err != nil {
		return rawprobe.Report{}, err
	}
	if decision := safety.EvaluateRawTest(d); !decision.Allowed || strings.TrimSpace(d.MajorMinor) == "" {
		return rawprobe.Report{}, ErrRawProbeProtected
	}
	if rawDeviceFingerprint(d) != record.fingerprint {
		return rawprobe.Report{}, ErrRawProbeIdentity
	}

	media, err := c.opener.Open(d.Path, d.MajorMinor)
	if err != nil {
		return rawprobe.Report{}, fmt.Errorf("open confirmed raw target: %w", err)
	}
	defer media.Close()

	// Second refresh: close the discovery/open TOCTOU window. The Linux opener
	// already verifies major:minor on the opened descriptor; refreshing again
	// catches a replacement/reinserted device or a mount that appeared after
	// the pre-open decision but before the first raw write.
	opened, err := c.refreshDevice(ctx, req.DeviceID)
	if err != nil {
		return rawprobe.Report{}, err
	}
	if decision := safety.EvaluateRawTest(opened); !decision.Allowed || strings.TrimSpace(opened.MajorMinor) == "" {
		return rawprobe.Report{}, ErrRawProbeProtected
	}
	if rawDeviceFingerprint(opened) != record.fingerprint || opened.Path != d.Path || opened.MajorMinor != d.MajorMinor {
		return rawprobe.Report{}, ErrRawProbeIdentity
	}

	seed, err := c.seed()
	if err != nil {
		return rawprobe.Report{}, fmt.Errorf("create raw probe seed: %w", err)
	}
	samples := req.Samples
	if samples < rawprobe.DefaultSamples {
		samples = rawprobe.DefaultSamples
	}
	report, runErr := c.engine.Run(ctx, media, rawprobe.Config{
		CapacityBytes: opened.SizeBytes,
		Samples:       samples,
		BlockBytes:    req.BlockBytes,
		Seed:          seed,
	}, func(progress rawprobe.Progress) {
		if emit != nil {
			emit(progress)
		}
	})
	if runErr != nil {
		return report, fmt.Errorf("raw capacity probe: %w", runErr)
	}
	return report, nil
}

func (c *RawProbeController) takeChallenge(req RawProbeRequest) (rawChallengeRecord, error) {
	if strings.TrimSpace(req.DeviceID) == "" || strings.TrimSpace(req.ChallengeToken) == "" {
		return rawChallengeRecord{}, ErrRawProbeChallenge
	}
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	record, ok := c.challenges[req.ChallengeToken]
	if !ok || !record.expires.After(now) || record.deviceID != req.DeviceID {
		delete(c.challenges, req.ChallengeToken)
		return rawChallengeRecord{}, ErrRawProbeChallenge
	}
	if req.Confirmation != record.confirmation {
		return rawChallengeRecord{}, ErrRawProbeConfirmation
	}
	delete(c.challenges, req.ChallengeToken)
	return record, nil
}

func (c *RawProbeController) refreshDevice(ctx context.Context, selectedID string) (device.Device, error) {
	items, err := c.devices.List(ctx)
	if err != nil {
		return device.Device{}, fmt.Errorf("refresh devices: %w", err)
	}
	for _, d := range items {
		if deviceID(d) == selectedID {
			return d, nil
		}
	}
	return device.Device{}, ErrDeviceNotFound
}

func rawDeviceFingerprint(d device.Device) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%d\x00%s\x00%s\x00%s", d.Path, d.KernelName, d.MajorMinor, d.Serial, d.SizeBytes, d.Vendor, d.Model, d.Transport)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func randomRawToken() (string, error) {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

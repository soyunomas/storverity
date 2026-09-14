package privhelper

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/rawprobe"
)

type fakeAuthorizer struct {
	err   error
	calls int
}

func (a *fakeAuthorizer) Authorize(context.Context, string, string, map[string]string) error {
	a.calls++
	return a.err
}

type sequenceDevices struct {
	lists [][]device.Device
	calls int
}

func (s *sequenceDevices) List(context.Context) ([]device.Device, error) {
	if len(s.lists) == 0 {
		return nil, nil
	}
	index := s.calls
	s.calls++
	if index >= len(s.lists) {
		index = len(s.lists) - 1
	}
	return s.lists[index], nil
}

type noopMedia struct{}

func (noopMedia) ReadAt([]byte, int64) (int, error)      { return 0, io.EOF }
func (noopMedia) WriteAt(p []byte, _ int64) (int, error) { return len(p), nil }
func (noopMedia) Sync() error                            { return nil }
func (noopMedia) Close() error                           { return nil }

type fakeOpener struct {
	path       string
	majorMinor string
	calls      int
	err        error
}

func (o *fakeOpener) Open(path, majorMinor string) (rawprobe.Media, error) {
	o.calls++
	o.path = path
	o.majorMinor = majorMinor
	if o.err != nil {
		return nil, o.err
	}
	return noopMedia{}, nil
}

type fakeEngine struct {
	cfg     rawprobe.Config
	report  rawprobe.Report
	err     error
	started chan struct{}
}

func (e *fakeEngine) Run(ctx context.Context, _ rawprobe.Media, cfg rawprobe.Config, _ func(rawprobe.Progress)) (rawprobe.Report, error) {
	e.cfg = cfg
	if e.started != nil {
		close(e.started)
		<-ctx.Done()
		return e.report, ctx.Err()
	}
	return e.report, e.err
}

func helperDevice() device.Device {
	return device.Device{
		Path: "/dev/sdb", KernelName: "sdb", MajorMinor: "8:16", Type: "disk",
		Serial: "USB123", SizeBytes: 64 * 1024 * 1024, Vendor: "Example", Model: "Flash",
		Transport: "usb", LikelyExternal: true,
	}
}

func helperRequest(d device.Device) RunRequest {
	return RunRequest{
		SessionID:           "session_0123456789abcdef",
		DeviceID:            device.ID(d),
		ExpectedFingerprint: device.Fingerprint(d),
		Samples:             8,
		BlockBytes:          512,
	}
}

func configuredServer(source DeviceSource, authorizer Authorizer, engine ProbeEngine, opener MediaOpener) *Server {
	server := NewServer(source, engine, opener, authorizer)
	server.seed = func() ([32]byte, error) { return [32]byte{9}, nil }
	return server
}

func TestServerRunsOnlyAfterAuthorizationAndDoubleIdentityCheck(t *testing.T) {
	dev := helperDevice()
	authorizer := &fakeAuthorizer{}
	engine := &fakeEngine{report: rawprobe.Report{Samples: rawprobe.DefaultSamples, Restored: true}}
	opener := &fakeOpener{}
	server := configuredServer(&sequenceDevices{lists: [][]device.Device{{dev}, {dev}}}, authorizer, engine, opener)

	response := server.RunRawProbe(context.Background(), ":1.42", helperRequest(dev), nil)
	if response.Error != nil {
		t.Fatalf("response error=%+v", response.Error)
	}
	if authorizer.calls != 1 || opener.calls != 1 {
		t.Fatalf("authorizer=%d opener=%d", authorizer.calls, opener.calls)
	}
	if opener.path != dev.Path || opener.majorMinor != dev.MajorMinor {
		t.Fatalf("opened %s %s", opener.path, opener.majorMinor)
	}
	if engine.cfg.CapacityBytes != dev.SizeBytes || engine.cfg.Samples != rawprobe.DefaultSamples || engine.cfg.BlockBytes != 512 || engine.cfg.Seed[0] != 9 {
		t.Fatalf("cfg=%+v", engine.cfg)
	}
}

func TestServerRejectsUnauthorizedCallerBeforeDeviceAccess(t *testing.T) {
	dev := helperDevice()
	authorizer := &fakeAuthorizer{err: ErrUnauthorized}
	source := &sequenceDevices{lists: [][]device.Device{{dev}}}
	server := configuredServer(source, authorizer, &fakeEngine{}, &fakeOpener{})

	response := server.RunRawProbe(context.Background(), ":1.50", helperRequest(dev), nil)
	if response.Error == nil || response.Error.Code != ErrorUnauthorized {
		t.Fatalf("response=%+v", response)
	}
	if source.calls != 0 {
		t.Fatalf("device source called %d times before authorization", source.calls)
	}
}

func TestServerRejectsStaleFingerprintBeforeOpen(t *testing.T) {
	dev := helperDevice()
	req := helperRequest(dev)
	req.ExpectedFingerprint = "stale-fingerprint"
	opener := &fakeOpener{}
	server := configuredServer(&sequenceDevices{lists: [][]device.Device{{dev}}}, &fakeAuthorizer{}, &fakeEngine{}, opener)

	response := server.RunRawProbe(context.Background(), ":1.51", req, nil)
	if response.Error == nil || response.Error.Code != ErrorIdentity {
		t.Fatalf("response=%+v", response)
	}
	if opener.calls != 0 {
		t.Fatalf("opener called %d times", opener.calls)
	}
}

func TestServerRejectsMountAppearingAfterOpen(t *testing.T) {
	before := helperDevice()
	after := before
	after.MountPoints = []string{"/media/USB"}
	opener := &fakeOpener{}
	server := configuredServer(&sequenceDevices{lists: [][]device.Device{{before}, {after}}}, &fakeAuthorizer{}, &fakeEngine{}, opener)

	response := server.RunRawProbe(context.Background(), ":1.52", helperRequest(before), nil)
	if response.Error == nil || response.Error.Code != ErrorProtected {
		t.Fatalf("response=%+v", response)
	}
	if opener.calls != 1 {
		t.Fatalf("opener calls=%d", opener.calls)
	}
}

func TestServerCancellationIsScopedToCallerAndSession(t *testing.T) {
	dev := helperDevice()
	engine := &fakeEngine{started: make(chan struct{})}
	server := configuredServer(&sequenceDevices{lists: [][]device.Device{{dev}, {dev}}}, &fakeAuthorizer{}, engine, &fakeOpener{})
	req := helperRequest(dev)
	result := make(chan RunResponse, 1)
	go func() {
		result <- server.RunRawProbe(context.Background(), ":1.60", req, nil)
	}()
	<-engine.started

	if server.Cancel(":1.61", req.SessionID) {
		t.Fatal("different caller must not cancel the active run")
	}
	if server.Cancel(":1.60", "wrong-session") {
		t.Fatal("different session must not cancel the active run")
	}
	if !server.Cancel(":1.60", req.SessionID) {
		t.Fatal("owner should cancel the active run")
	}
	response := <-result
	if response.Error == nil || response.Error.Code != ErrorCancelled {
		t.Fatalf("response=%+v", response)
	}
}

func TestServerRejectsInvalidGeometry(t *testing.T) {
	dev := helperDevice()
	req := helperRequest(dev)
	req.BlockBytes = 777
	server := configuredServer(&sequenceDevices{lists: [][]device.Device{{dev}, {dev}}}, &fakeAuthorizer{}, &fakeEngine{}, &fakeOpener{})
	response := server.RunRawProbe(context.Background(), ":1.70", req, nil)
	if response.Error == nil || !errors.Is(clientError(response.Error), ErrInvalid) {
		t.Fatalf("response=%+v", response)
	}
}

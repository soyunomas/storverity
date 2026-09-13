package appservice

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/rawprobe"
)

type fakeRawEngine struct {
	cfg      rawprobe.Config
	report   rawprobe.Report
	err      error
	progress []rawprobe.Progress
	calls    int
}

func (f *fakeRawEngine) Run(_ context.Context, _ rawprobe.Media, cfg rawprobe.Config, emit func(rawprobe.Progress)) (rawprobe.Report, error) {
	f.calls++
	f.cfg = cfg
	for _, progress := range f.progress {
		emit(progress)
	}
	return f.report, f.err
}

type stubRawMedia struct{}

func (stubRawMedia) ReadAt([]byte, int64) (int, error)  { return 0, io.EOF }
func (stubRawMedia) WriteAt(p []byte, _ int64) (int, error) { return len(p), nil }
func (stubRawMedia) Sync() error                         { return nil }
func (stubRawMedia) Close() error                        { return nil }

type fakeRawOpener struct {
	path     string
	majorMin string
	calls    int
	err      error
}

func (f *fakeRawOpener) Open(path, majorMin string) (rawprobe.Media, error) {
	f.calls++
	f.path = path
	f.majorMin = majorMin
	if f.err != nil {
		return nil, f.err
	}
	return stubRawMedia{}, nil
}

type sequenceSource struct {
	lists [][]device.Device
	calls int
}

func (s *sequenceSource) List(context.Context) ([]device.Device, error) {
	index := s.calls
	s.calls++
	if index >= len(s.lists) {
		index = len(s.lists) - 1
	}
	return s.lists[index], nil
}

func eligibleRawDevice() device.Device {
	return device.Device{
		Path: "/dev/sdb", KernelName: "sdb", MajorMinor: "8:16", Type: "disk",
		Serial: "USB123", SizeBytes: 64 * 1024 * 1024, Vendor: "Example", Model: "Flash",
		Transport: "usb", LikelyExternal: true,
	}
}

func configuredRawController(source DeviceSource, engine RawProbeEngine, opener RawMediaOpener) *RawProbeController {
	controller := NewRawProbeController(source, engine, opener)
	controller.token = func() (string, error) { return "challenge-token", nil }
	controller.seed = func() ([32]byte, error) { return [32]byte{7}, nil }
	controller.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	return controller
}

func TestRawProbePrepareAndRunRefreshesSafetyAndIdentity(t *testing.T) {
	dev := eligibleRawDevice()
	engine := &fakeRawEngine{report: rawprobe.Report{Samples: 8, ValidSamples: 8, Restored: true}}
	opener := &fakeRawOpener{}
	controller := configuredRawController(fakeSource{devices: []device.Device{dev}}, engine, opener)

	challenge, err := controller.Prepare(context.Background(), "serial:USB123")
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Token != "challenge-token" || challenge.ConfirmationText != "DESTROY DATA ON /dev/sdb" {
		t.Fatalf("challenge=%+v", challenge)
	}

	var progress []RawProbeProgress
	report, err := controller.Run(context.Background(), RawProbeRequest{
		DeviceID: "serial:USB123", ChallengeToken: challenge.Token,
		Confirmation: challenge.ConfirmationText, Samples: 8, BlockBytes: 512,
	}, func(p RawProbeProgress) { progress = append(progress, p) })
	if err != nil {
		t.Fatal(err)
	}
	if report.ValidSamples != 8 || engine.calls != 1 {
		t.Fatalf("report=%+v calls=%d", report, engine.calls)
	}
	if engine.cfg.CapacityBytes != dev.SizeBytes || engine.cfg.Samples != 8 || engine.cfg.BlockBytes != 512 || engine.cfg.Seed[0] != 7 {
		t.Fatalf("cfg=%+v", engine.cfg)
	}
	if opener.calls != 1 || opener.path != "/dev/sdb" || opener.majorMin != "8:16" {
		t.Fatalf("opener=%+v", opener)
	}
}

func TestRawProbeRejectsStaleIdentityAfterConfirmation(t *testing.T) {
	before := eligibleRawDevice()
	after := before
	after.MajorMinor = "8:32"
	source := &sequenceSource{lists: [][]device.Device{{before}, {after}}}
	controller := configuredRawController(source, &fakeRawEngine{}, &fakeRawOpener{})
	challenge, err := controller.Prepare(context.Background(), "serial:USB123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Run(context.Background(), RawProbeRequest{
		DeviceID: "serial:USB123", ChallengeToken: challenge.Token, Confirmation: challenge.ConfirmationText,
	}, nil)
	if !errors.Is(err, ErrRawProbeIdentity) {
		t.Fatalf("err=%v", err)
	}
}

func TestRawProbeRejectsMountedTargetBeforeChallenge(t *testing.T) {
	dev := eligibleRawDevice()
	dev.MountPoints = []string{"/media/USB"}
	controller := configuredRawController(fakeSource{devices: []device.Device{dev}}, &fakeRawEngine{}, &fakeRawOpener{})
	_, err := controller.Prepare(context.Background(), "serial:USB123")
	if !errors.Is(err, ErrRawProbeProtected) {
		t.Fatalf("err=%v", err)
	}
}

func TestRawProbeRequiresExactConfirmationAndSingleUseToken(t *testing.T) {
	dev := eligibleRawDevice()
	engine := &fakeRawEngine{}
	controller := configuredRawController(fakeSource{devices: []device.Device{dev}}, engine, &fakeRawOpener{})
	challenge, err := controller.Prepare(context.Background(), "serial:USB123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Run(context.Background(), RawProbeRequest{
		DeviceID: "serial:USB123", ChallengeToken: challenge.Token, Confirmation: "yes",
	}, nil)
	if !errors.Is(err, ErrRawProbeConfirmation) {
		t.Fatalf("err=%v", err)
	}
	_, err = controller.Run(context.Background(), RawProbeRequest{
		DeviceID: "serial:USB123", ChallengeToken: challenge.Token, Confirmation: challenge.ConfirmationText,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Run(context.Background(), RawProbeRequest{
		DeviceID: "serial:USB123", ChallengeToken: challenge.Token, Confirmation: challenge.ConfirmationText,
	}, nil)
	if !errors.Is(err, ErrRawProbeChallenge) {
		t.Fatalf("reused token err=%v", err)
	}
}

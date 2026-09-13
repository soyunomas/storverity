package appservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/verifyfs"
)

type contextSource struct {
	devices []device.Device
	err     error
	block   bool
}

func (s contextSource) List(ctx context.Context) ([]device.Device, error) {
	if s.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return s.devices, s.err
}

func TestDesktopListDevices(t *testing.T) {
	desktop := NewDesktop(contextSource{devices: []device.Device{{
		Path: "/dev/sdb", KernelName: "sdb", Type: "disk", Serial: "USB1",
		SizeBytes: 64_000, LikelyExternal: true,
	}}}, &fakeVerifier{}, nil)

	cards, err := desktop.ListDevices()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].ID != "serial:USB1" {
		t.Fatalf("cards=%+v", cards)
	}
}

func TestDesktopListDevicesUsesTimeout(t *testing.T) {
	desktop := NewDesktop(contextSource{block: true}, &fakeVerifier{}, nil)
	desktop.listTimeout = 5 * time.Millisecond
	_, err := desktop.ListDevices()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v, want deadline exceeded", err)
	}
}

func TestDesktopVerificationDelegatesProgressAndCancellation(t *testing.T) {
	runner := &blockingRunner{started: make(chan struct{})}
	desktop := &Desktop{
		devices:      New(contextSource{}),
		verification: NewVerificationManager(runner),
	}
	result := make(chan error, 1)
	go func() {
		_, err := desktop.StartVerification(VerificationRequest{})
		result <- err
	}()
	<-runner.started

	if !desktop.VerificationActive() {
		t.Fatal("VerificationActive() = false")
	}
	if !desktop.CancelVerification() {
		t.Fatal("CancelVerification() = false")
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestDesktopEmitsProgress(t *testing.T) {
	source := contextSource{devices: []device.Device{{
		Path: "/dev/sdb", KernelName: "sdb", Type: "disk", SizeBytes: 1,
		LikelyExternal: true, MountPoints: []string{"/media/USB"},
	}}}
	engine := &fakeVerifier{
		report: verifyfs.Report{Regions: 1, BytesWritten: 1, BytesVerified: 1},
		progress: []verifyfs.Progress{{Phase: verifyfs.PhaseWrite, Region: 0, RegionsTotal: 1, BytesCompleted: 1, BytesTotal: 1}},
	}
	var got []VerificationProgress
	desktop := NewDesktop(source, engine, func(p VerificationProgress) { got = append(got, p) })
	_, err := desktop.StartVerification(VerificationRequest{DeviceID: "kernel:sdb", MountPoint: "/media/USB", TotalBytes: 1, ChunkBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Region != 0 || got[0].Phase != verifyfs.PhaseWrite {
		t.Fatalf("progress=%+v", got)
	}
}

func TestDesktopRejectsMissingConfiguration(t *testing.T) {
	var desktop *Desktop
	if _, err := desktop.ListDevices(); err == nil {
		t.Fatal("ListDevices() error = nil")
	}
	if _, err := desktop.StartVerification(VerificationRequest{}); err == nil {
		t.Fatal("StartVerification() error = nil")
	}
	if desktop.CancelVerification() || desktop.VerificationActive() {
		t.Fatal("nil desktop reported active/cancelled verification")
	}
}

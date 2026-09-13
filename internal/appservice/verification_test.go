package appservice

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/verifyfs"
)

type fakeVerifier struct {
	cfg      verifyfs.Config
	report   verifyfs.Report
	err      error
	progress []verifyfs.Progress
	calls    int
}

func (f *fakeVerifier) Run(_ context.Context, cfg verifyfs.Config, emit func(verifyfs.Progress)) (verifyfs.Report, error) {
	f.calls++
	f.cfg = cfg
	for _, progress := range f.progress {
		emit(progress)
	}
	return f.report, f.err
}

func TestVerificationControllerRefreshesIdentityAndRunsMountedExternalDevice(t *testing.T) {
	source := fakeSource{devices: []device.Device{{
		Path: "/dev/sdb", KernelName: "sdb", Type: "disk", Serial: "USB123", SizeBytes: 64_000,
		LikelyExternal: true, MountPoints: []string{"/media/USB"},
	}}}
	engine := &fakeVerifier{
		report:   verifyfs.Report{BytesWritten: 128, BytesVerified: 128, Regions: 2},
		progress: []verifyfs.Progress{{Phase: verifyfs.PhaseWrite, Region: 0, RegionsTotal: 2, BytesCompleted: 64, BytesTotal: 128}},
	}
	controller := NewVerificationController(source, engine)
	controller.seed = func() ([32]byte, error) { var s [32]byte; s[0] = 7; return s, nil }

	var events []VerificationProgress
	report, err := controller.Run(context.Background(), VerificationRequest{
		DeviceID: "serial:USB123", MountPoint: "/media/USB", TotalBytes: 128, ChunkBytes: 64,
	}, func(progress VerificationProgress) { events = append(events, progress) })
	if err != nil {
		t.Fatal(err)
	}
	if engine.calls != 1 {
		t.Fatalf("calls=%d", engine.calls)
	}
	if engine.cfg.Root != "/media/USB" || engine.cfg.TotalBytes != 128 || engine.cfg.ChunkBytes != 64 || engine.cfg.Seed[0] != 7 {
		t.Fatalf("cfg=%+v", engine.cfg)
	}
	if report.Regions != 2 {
		t.Fatalf("report=%+v", report)
	}
	if want := []VerificationProgress{{Phase: verifyfs.PhaseWrite, Region: 0, RegionsTotal: 2, BytesCompleted: 64, BytesTotal: 128}}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%+v want=%+v", events, want)
	}
}

func TestVerificationControllerRejectsStaleDevice(t *testing.T) {
	controller := NewVerificationController(fakeSource{}, &fakeVerifier{})
	_, err := controller.Run(context.Background(), VerificationRequest{DeviceID: "serial:gone", MountPoint: "/media/USB", TotalBytes: 1, ChunkBytes: 1}, nil)
	if !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestVerificationControllerRejectsStaleMount(t *testing.T) {
	controller := NewVerificationController(fakeSource{devices: []device.Device{{Path: "/dev/sdb", KernelName: "sdb", Type: "disk", Serial: "USB123", SizeBytes: 1, LikelyExternal: true, MountPoints: []string{"/media/OTHER"}}}}, &fakeVerifier{})
	_, err := controller.Run(context.Background(), VerificationRequest{DeviceID: "serial:USB123", MountPoint: "/media/USB", TotalBytes: 1, ChunkBytes: 1}, nil)
	if !errors.Is(err, ErrMountNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestVerificationControllerRejectsProtectedTargets(t *testing.T) {
	tests := []device.Device{
		{Path: "/dev/nvme0n1", KernelName: "nvme0n1", Type: "disk", SizeBytes: 1, SystemDisk: true, LikelyExternal: true, MountPoints: []string{"/"}},
		{Path: "/dev/sdb", KernelName: "sdb", Type: "disk", SizeBytes: 1, ReadOnly: true, LikelyExternal: true, MountPoints: []string{"/media/USB"}},
		{Path: "/dev/sda", KernelName: "sda", Type: "disk", SizeBytes: 1, LikelyExternal: false, MountPoints: []string{"/mnt/data"}},
	}
	for _, d := range tests {
		controller := NewVerificationController(fakeSource{devices: []device.Device{d}}, &fakeVerifier{})
		_, err := controller.Run(context.Background(), VerificationRequest{DeviceID: deviceID(d), MountPoint: d.MountPoints[0], TotalBytes: 1, ChunkBytes: 1}, nil)
		if !errors.Is(err, ErrTargetProtected) {
			t.Fatalf("device=%+v err=%v", d, err)
		}
	}
}

func TestVerificationControllerPropagatesEngineError(t *testing.T) {
	sentinel := errors.New("disk full")
	engine := &fakeVerifier{err: sentinel}
	controller := NewVerificationController(fakeSource{devices: []device.Device{{Path: "/dev/sdb", KernelName: "sdb", Type: "disk", SizeBytes: 1, LikelyExternal: true, MountPoints: []string{"/media/USB"}}}}, engine)
	_, err := controller.Run(context.Background(), VerificationRequest{DeviceID: "kernel:sdb", MountPoint: "/media/USB", TotalBytes: 1, ChunkBytes: 1}, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v", err)
	}
}

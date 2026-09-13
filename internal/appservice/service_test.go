package appservice

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/safety"
)

type fakeSource struct {
	devices []device.Device
	err     error
}

func (f fakeSource) List(context.Context) ([]device.Device, error) { return f.devices, f.err }

func TestListDevicesMapsAndOrdersExternalFirst(t *testing.T) {
	svc := New(fakeSource{devices: []device.Device{
		{Path: "/dev/nvme0n1", KernelName: "nvme0n1", Type: "disk", Model: "System SSD", SizeBytes: 1_000, SystemDisk: true, MountPoints: []string{"/"}},
		{Path: "/dev/sdb", KernelName: "sdb", Type: "disk", Vendor: "Example", Model: "Flash", Serial: "USB123", Transport: "usb", SizeBytes: 64_000, LikelyExternal: true, MountPoints: []string{"/media/USB"}, FileSystems: []string{"exfat"}},
	}})
	cards, err := svc.ListDevices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("len=%d", len(cards))
	}
	usb := cards[0]
	if usb.Path != "/dev/sdb" || usb.ID != "serial:USB123" || usb.DisplayName != "Example Flash" {
		t.Fatalf("usb=%+v", usb)
	}
	if usb.RawTest.Allowed {
		t.Fatal("mounted USB must not be raw-test eligible")
	}
	if !hasReason(usb.RawTest, "mounted") {
		t.Fatalf("reasons=%+v", usb.RawTest.Reasons)
	}
	if got, want := usb.MountPoints, []string{"/media/USB"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mounts=%v want=%v", got, want)
	}
	if !cards[1].SystemDisk {
		t.Fatalf("system=%+v", cards[1])
	}
}

func TestListDevicesUsesStableFallbackIdentityAndNonNilSlices(t *testing.T) {
	svc := New(fakeSource{devices: []device.Device{{Path: "/dev/sdc", KernelName: "sdc", Type: "disk", SizeBytes: 1, LikelyExternal: true}}})
	cards, err := svc.ListDevices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := cards[0]
	if got.ID != "kernel:sdc" || got.DisplayName != "/dev/sdc" {
		t.Fatalf("card=%+v", got)
	}
	if got.MountPoints == nil || got.FileSystems == nil {
		t.Fatal("slices must serialize as [] instead of null")
	}
}

func TestListDevicesWrapsSourceError(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := New(fakeSource{err: sentinel}).ListDevices(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v", err)
	}
}
func TestListDevicesRejectsMissingSource(t *testing.T) {
	if _, err := New(nil).ListDevices(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
func TestDisplayNameFallback(t *testing.T) {
	if got := displayName(device.Device{}); got != "Unknown storage device" {
		t.Fatalf("got=%q", got)
	}
}
func hasReason(d safety.Decision, code string) bool {
	for _, r := range d.Reasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

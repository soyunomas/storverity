package safety

import (
	"testing"

	"github.com/soyunomas/storverity/internal/device"
)

func TestEvaluateRawTestAllowsSafeUSB(t *testing.T) {
	d := device.Device{
		Path:           "/dev/sdb",
		Type:           "disk",
		SizeBytes:      64_000_000_000,
		Transport:      "usb",
		LikelyExternal: true,
		Serial:         "ABC123",
	}

	got := EvaluateRawTest(d)
	if !got.Allowed {
		t.Fatalf("Allowed = false, reasons = %#v", got.Reasons)
	}
	if len(got.Reasons) != 0 {
		t.Fatalf("reasons = %#v, want none", got.Reasons)
	}
}

func TestEvaluateRawTestDeniesDangerousDevice(t *testing.T) {
	d := device.Device{
		Path:         "/dev/nvme0n1",
		Type:         "disk",
		SizeBytes:    1_000_000_000_000,
		SystemDisk:   true,
		ContainsSwap: true,
		MountPoints:  []string{"/", "/boot"},
		Serial:       "SYS1",
	}

	got := EvaluateRawTest(d)
	if got.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	for _, code := range []string{"system_disk", "contains_swap", "mounted"} {
		if !hasReason(got, code, SeverityDeny) {
			t.Fatalf("missing deny reason %q in %#v", code, got.Reasons)
		}
	}
}

func TestEvaluateRawTestDeniesReadOnlyMountedMedia(t *testing.T) {
	d := device.Device{
		Path:           "/dev/sdc",
		Type:           "disk",
		SizeBytes:      32_000_000_000,
		ReadOnly:       true,
		MountPoints:    []string{"/media/user/card"},
		LikelyExternal: true,
		Serial:         "CARD1",
	}

	got := EvaluateRawTest(d)
	if got.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if !hasReason(got, "read_only", SeverityDeny) || !hasReason(got, "mounted", SeverityDeny) {
		t.Fatalf("reasons = %#v", got.Reasons)
	}
}

func TestEvaluateRawTestDeniesNonExternalAndWarnsForWeakIdentity(t *testing.T) {
	d := device.Device{
		Path:      "/dev/sdd",
		Type:      "disk",
		SizeBytes: 1_000_000_000,
	}

	got := EvaluateRawTest(d)
	if got.Allowed {
		t.Fatalf("Allowed = true, reasons = %#v", got.Reasons)
	}
	if !hasReason(got, "not_likely_external", SeverityDeny) || !hasReason(got, "missing_serial", SeverityWarning) {
		t.Fatalf("reasons = %#v", got.Reasons)
	}
}

func TestEvaluateRawTestRejectsInvalidShape(t *testing.T) {
	d := device.Device{Path: "sdb1", Type: "part"}
	got := EvaluateRawTest(d)
	for _, code := range []string{"not_whole_disk", "invalid_device_path", "unknown_capacity"} {
		if !hasReason(got, code, SeverityDeny) {
			t.Fatalf("missing reason %q in %#v", code, got.Reasons)
		}
	}
}

func hasReason(d Decision, code string, severity Severity) bool {
	for _, reason := range d.Reasons {
		if reason.Code == code && reason.Severity == severity {
			return true
		}
	}
	return false
}

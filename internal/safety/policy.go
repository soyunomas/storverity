package safety

import (
	"strings"

	"github.com/soyunomas/storverity/internal/device"
)

type Severity string

const (
	SeverityDeny    Severity = "deny"
	SeverityWarning Severity = "warning"
)

type Reason struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

type Decision struct {
	Allowed bool     `json:"allowed"`
	Reasons []Reason `json:"reasons,omitempty"`
}

// EvaluateRawTest determines whether a whole disk is eligible for a future
// destructive raw-capacity test. This function only evaluates metadata; it
// performs no I/O and grants no persistent authorization.
//
// The raw engine must request a fresh decision immediately before opening a
// device for writing because mount state and device identity can change.
func EvaluateRawTest(d device.Device) Decision {
	reasons := make([]Reason, 0, 8)
	deny := func(code, message string) {
		reasons = append(reasons, Reason{Code: code, Severity: SeverityDeny, Message: message})
	}
	warn := func(code, message string) {
		reasons = append(reasons, Reason{Code: code, Severity: SeverityWarning, Message: message})
	}

	if d.Type != "disk" {
		deny("not_whole_disk", "The target is not a whole-disk block device.")
	}
	if !strings.HasPrefix(d.Path, "/dev/") || strings.TrimSpace(d.Path) == "/dev/" {
		deny("invalid_device_path", "The target does not have a valid Linux block-device path.")
	}
	if d.SizeBytes == 0 {
		deny("unknown_capacity", "The device reports zero or unknown capacity.")
	}
	if d.ReadOnly {
		deny("read_only", "The device is read-only.")
	}
	if d.SystemDisk {
		deny("system_disk", "The device contains a critical system mount point.")
	}
	if d.ContainsSwap {
		deny("contains_swap", "The device contains a swap filesystem.")
	}
	if len(d.MountPoints) != 0 {
		deny("mounted", "The device or one of its descendants is mounted.")
	}
	if !d.LikelyExternal {
		deny("not_likely_external", "The device is not identified as removable or externally attached.")
	}
	if strings.TrimSpace(d.Serial) == "" {
		warn("missing_serial", "The device has no serial number, making identity checks less reliable.")
	}

	allowed := true
	for _, reason := range reasons {
		if reason.Severity == SeverityDeny {
			allowed = false
			break
		}
	}
	return Decision{Allowed: allowed, Reasons: reasons}
}

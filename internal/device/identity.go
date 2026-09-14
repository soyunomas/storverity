package device

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// ID returns the stable selection key used by the desktop and privileged
// helper. Serial is preferred, followed by the kernel name and finally path.
func ID(d Device) string {
	if serial := strings.TrimSpace(d.Serial); serial != "" {
		return "serial:" + serial
	}
	if kernel := strings.TrimSpace(d.KernelName); kernel != "" {
		return "kernel:" + kernel
	}
	return "path:" + strings.TrimSpace(d.Path)
}

// Fingerprint binds a user confirmation to the concrete device identity and
// geometry observed during discovery. It is intentionally derived from fields
// that the privileged helper can independently rediscover before writing.
func Fingerprint(d Device) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%d\x00%s\x00%s\x00%s", d.Path, d.KernelName, d.MajorMinor, d.Serial, d.SizeBytes, d.Vendor, d.Model, d.Transport)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

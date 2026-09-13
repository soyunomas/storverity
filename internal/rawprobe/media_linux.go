//go:build linux

package rawprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// OpenLinuxBlockDevice opens a block device for synchronous read/write access
// and verifies its major:minor identity after opening. Callers must perform a
// fresh discovery/safety decision before this function; the post-open device
// number check closes the path-replacement window between discovery and open.
func OpenLinuxBlockDevice(path string, expectedMajorMinor string) (Media, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean != path || !strings.HasPrefix(clean, "/dev/") || clean == "/dev" {
		return nil, fmt.Errorf("invalid raw device path %q", path)
	}
	f, err := os.OpenFile(clean, os.O_RDWR|os.O_SYNC, 0)
	if err != nil {
		return nil, fmt.Errorf("open raw device %s: %w", clean, err)
	}
	fail := func(err error) (Media, error) {
		_ = f.Close()
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		return fail(fmt.Errorf("stat opened raw device: %w", err))
	}
	if info.Mode()&os.ModeDevice == 0 || info.Mode()&os.ModeCharDevice != 0 {
		return fail(fmt.Errorf("%s is not a block device", clean))
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fail(fmt.Errorf("cannot inspect Linux device identity for %s", clean))
	}
	actual := fmt.Sprintf("%d:%d", unix.Major(uint64(st.Rdev)), unix.Minor(uint64(st.Rdev)))
	if expectedMajorMinor == "" || actual != expectedMajorMinor {
		return fail(fmt.Errorf("block-device identity changed: expected %q, opened %q", expectedMajorMinor, actual))
	}
	return f, nil
}

package appservice

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/soyunomas/storverity/internal/device"
)

const linuxMountInfoPath = "/proc/self/mountinfo"

type mountAwareSource struct {
	base          DeviceSource
	mountInfoPath string
}

func newLinuxMountAwareSource(base DeviceSource) DeviceSource {
	return &mountAwareSource{base: base, mountInfoPath: linuxMountInfoPath}
}

func (s *mountAwareSource) List(ctx context.Context) ([]device.Device, error) {
	if s == nil || s.base == nil {
		return nil, fmt.Errorf("device source is not configured")
	}
	items, err := s.base.List(ctx)
	if err != nil {
		return nil, err
	}
	modes, err := readMountModes(s.mountInfoPath)
	if err != nil {
		return nil, fmt.Errorf("read mount table: %w", err)
	}
	for i := range items {
		applyMountModes(&items[i], modes)
	}
	return items, nil
}

type mountMode struct {
	readOnly bool
}

func readMountModes(path string) (map[string]mountMode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseMountInfo(data)
}

func parseMountInfo(data []byte) (map[string]mountMode, error) {
	modes := make(map[string]mountMode)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue
		}
		mountPoint := decodeMountInfoPath(fields[4])
		if mountPoint == "" {
			continue
		}
		mode := mountMode{readOnly: true}
		for _, option := range strings.Split(fields[5], ",") {
			switch option {
			case "rw":
				mode.readOnly = false
			case "ro":
				mode.readOnly = true
			}
		}
		modes[mountPoint] = mode
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return modes, nil
}

func decodeMountInfoPath(value string) string {
	return strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	).Replace(value)
}

func applyMountModes(d *device.Device, modes map[string]mountMode) {
	if d == nil {
		return
	}
	writable := make([]string, 0, len(d.MountPoints))
	other := make([]string, 0, len(d.MountPoints))
	for _, mountPoint := range d.MountPoints {
		mode, ok := modes[mountPoint]
		if ok && !mode.readOnly {
			writable = append(writable, mountPoint)
			continue
		}
		other = append(other, mountPoint)
	}
	sort.Strings(writable)
	sort.Strings(other)
	d.WritableMountPoints = append([]string(nil), writable...)
	d.MountPoints = append(writable, other...)
}

//go:build linux

package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// Scanner discovers Linux block devices without opening them for I/O.
type Scanner struct {
	lsblkPath string
}

func NewScanner() *Scanner {
	return &Scanner{lsblkPath: "lsblk"}
}

// List returns whole-disk devices. Descendants are used to derive mount,
// filesystem, swap and system-storage metadata for their physical parent.
func (s *Scanner) List(ctx context.Context) ([]Device, error) {
	args := []string{
		"--json", "--bytes", "--paths",
		"--output", "NAME,KNAME,PATH,MAJ:MIN,TYPE,TRAN,RM,RO,SIZE,MODEL,VENDOR,SERIAL,MOUNTPOINTS,FSTYPE",
	}

	out, err := exec.CommandContext(ctx, s.lsblkPath, args...).Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("run lsblk: %w", err)
	}

	return parseLSBLK(out)
}

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name        string        `json:"name"`
	KernelName  string        `json:"kname"`
	Path        string        `json:"path"`
	MajorMinor  string        `json:"maj:min"`
	Type        string        `json:"type"`
	Transport   string        `json:"tran"`
	Removable   boolish       `json:"rm"`
	ReadOnly    boolish       `json:"ro"`
	Size        uint64ish     `json:"size"`
	Model       string        `json:"model"`
	Vendor      string        `json:"vendor"`
	Serial      string        `json:"serial"`
	MountPoints []interface{} `json:"mountpoints"`
	FileSystem  string        `json:"fstype"`
	Children    []lsblkDevice `json:"children"`
}

// util-linux can encode 0/1 fields as JSON booleans, numbers, or strings,
// depending on version/options. These decoders tolerate those representations.
type boolish bool

func (b *boolish) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	switch s {
	case "true", "1", `"1"`:
		*b = true
		return nil
	case "false", "0", `"0"`, "null", `""`:
		*b = false
		return nil
	default:
		return fmt.Errorf("invalid boolean value %s", s)
	}
}

type uint64ish uint64

func (u *uint64ish) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if s == "" || s == "null" {
		*u = 0
		return nil
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid uint64 value %q: %w", s, err)
	}
	*u = uint64ish(n)
	return nil
}

func parseLSBLK(data []byte) ([]Device, error) {
	var raw lsblkOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode lsblk JSON: %w", err)
	}

	devices := make([]Device, 0, len(raw.BlockDevices))
	for _, d := range raw.BlockDevices {
		if strings.TrimSpace(d.Type) != "disk" {
			continue
		}

		meta := descendantMetadata{
			mounts:      stringSet{},
			filesystems: stringSet{},
		}
		collectMetadata(d, &meta)

		transport := strings.ToLower(strings.TrimSpace(d.Transport))
		dev := Device{
			Name:           strings.TrimSpace(d.Name),
			KernelName:     strings.TrimSpace(d.KernelName),
			Path:           strings.TrimSpace(d.Path),
			MajorMinor:     strings.TrimSpace(d.MajorMinor),
			Type:           strings.TrimSpace(d.Type),
			Transport:      transport,
			Removable:      bool(d.Removable),
			ReadOnly:       bool(d.ReadOnly),
			SizeBytes:      uint64(d.Size),
			Model:          strings.TrimSpace(d.Model),
			Vendor:         strings.TrimSpace(d.Vendor),
			Serial:         strings.TrimSpace(d.Serial),
			MountPoints:    meta.mounts.sorted(),
			FileSystems:    meta.filesystems.sorted(),
			ContainsSwap:   meta.containsSwap,
			LikelyExternal: likelyExternal(bool(d.Removable), transport),
		}
		dev.SystemDisk = containsCriticalSystemMount(dev.MountPoints)
		devices = append(devices, dev)
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Path < devices[j].Path
	})
	return devices, nil
}

type descendantMetadata struct {
	mounts       stringSet
	filesystems  stringSet
	containsSwap bool
}

type stringSet map[string]struct{}

func (s stringSet) add(value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		s[value] = struct{}{}
	}
}

func (s stringSet) sorted() []string {
	out := make([]string, 0, len(s))
	for value := range s {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func collectMetadata(d lsblkDevice, meta *descendantMetadata) {
	for _, value := range d.MountPoints {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok {
			meta.mounts.add(text)
		}
	}

	fs := strings.TrimSpace(d.FileSystem)
	meta.filesystems.add(fs)
	if strings.EqualFold(fs, "swap") {
		meta.containsSwap = true
	}

	for _, child := range d.Children {
		collectMetadata(child, meta)
	}
}

func containsCriticalSystemMount(mounts []string) bool {
	for _, mount := range mounts {
		switch mount {
		case "/", "/boot", "/boot/efi", "/home", "/usr", "/var":
			return true
		}
	}
	return false
}

func likelyExternal(removable bool, transport string) bool {
	if removable {
		return true
	}
	switch transport {
	case "usb", "ieee1394", "thunderbolt":
		return true
	default:
		return false
	}
}

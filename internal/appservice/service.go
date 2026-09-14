package appservice

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/safety"
)

// DeviceSource is deliberately tiny so the desktop application can be tested
// without invoking lsblk or depending on Wails.
type DeviceSource interface {
	List(context.Context) ([]device.Device, error)
}

type DeviceCard struct {
	ID                  string          `json:"id"`
	Path                string          `json:"path"`
	DisplayName         string          `json:"displayName"`
	Vendor              string          `json:"vendor,omitempty"`
	Model               string          `json:"model,omitempty"`
	Serial              string          `json:"serial,omitempty"`
	Transport           string          `json:"transport,omitempty"`
	CapacityBytes       uint64          `json:"capacityBytes"`
	MountPoints         []string        `json:"mountPoints"`
	WritableMountPoints []string        `json:"writableMountPoints"`
	FileSystems         []string        `json:"fileSystems"`
	ReadOnly            bool            `json:"readOnly"`
	SystemDisk          bool            `json:"systemDisk"`
	LikelyExternal      bool            `json:"likelyExternal"`
	RawTest             safety.Decision `json:"rawTest"`
}

type Service struct{ devices DeviceSource }

func New(devices DeviceSource) *Service { return &Service{devices: devices} }
func NewLinux() *Service                { return New(device.NewScanner()) }

func (s *Service) ListDevices(ctx context.Context) ([]DeviceCard, error) {
	if s == nil || s.devices == nil {
		return nil, fmt.Errorf("device source is not configured")
	}
	items, err := s.devices.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	cards := make([]DeviceCard, 0, len(items))
	for _, d := range items {
		cards = append(cards, mapDevice(d))
	}
	sort.SliceStable(cards, func(i, j int) bool {
		if cards[i].LikelyExternal != cards[j].LikelyExternal {
			return cards[i].LikelyExternal
		}
		return cards[i].Path < cards[j].Path
	})
	return cards, nil
}

func mapDevice(d device.Device) DeviceCard {
	mounts := append([]string(nil), d.MountPoints...)
	writableMounts := append([]string(nil), d.WritableMountPoints...)
	filesystems := append([]string(nil), d.FileSystems...)
	if mounts == nil {
		mounts = []string{}
	}
	if writableMounts == nil {
		writableMounts = []string{}
	}
	if filesystems == nil {
		filesystems = []string{}
	}
	return DeviceCard{
		ID: device.ID(d), Path: d.Path, DisplayName: displayName(d), Vendor: d.Vendor, Model: d.Model, Serial: d.Serial,
		Transport: d.Transport, CapacityBytes: d.SizeBytes, MountPoints: mounts, WritableMountPoints: writableMounts, FileSystems: filesystems,
		ReadOnly: d.ReadOnly, SystemDisk: d.SystemDisk, LikelyExternal: d.LikelyExternal, RawTest: safety.EvaluateRawTest(d),
	}
}

func displayName(d device.Device) string {
	parts := make([]string, 0, 2)
	if v := strings.TrimSpace(d.Vendor); v != "" {
		parts = append(parts, v)
	}
	if m := strings.TrimSpace(d.Model); m != "" {
		parts = append(parts, m)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	if p := strings.TrimSpace(d.Path); p != "" {
		return p
	}
	return "Unknown storage device"
}

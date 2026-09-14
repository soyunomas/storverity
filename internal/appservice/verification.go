package appservice

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/verifyfs"
)

var (
	ErrDeviceNotFound   = errors.New("selected device is no longer present")
	ErrMountNotFound    = errors.New("selected mount point no longer belongs to the device")
	ErrMountNotWritable = errors.New("selected mount point is not writable")
	ErrTargetProtected  = errors.New("selected device is not eligible for filesystem verification")
)

type FilesystemVerifier interface {
	Run(context.Context, verifyfs.Config, func(verifyfs.Progress)) (verifyfs.Report, error)
}

type VerificationRequest struct {
	DeviceID   string `json:"deviceId"`
	MountPoint string `json:"mountPoint"`
	TotalBytes int64  `json:"totalBytes"`
	ChunkBytes int64  `json:"chunkBytes"`
}

type VerificationProgress struct {
	Phase          verifyfs.Phase         `json:"phase"`
	Region         int                    `json:"region"`
	RegionsTotal   int                    `json:"regionsTotal"`
	BytesCompleted int64                  `json:"bytesCompleted"`
	BytesTotal     int64                  `json:"bytesTotal"`
	Outcome        verifyfs.RegionOutcome `json:"outcome"`
	Error          string                 `json:"error,omitempty"`
}

type VerificationController struct {
	devices DeviceSource
	engine  FilesystemVerifier
	seed    func() ([32]byte, error)
}

func NewVerificationController(devices DeviceSource, engine FilesystemVerifier) *VerificationController {
	return &VerificationController{devices: devices, engine: engine, seed: randomSeed}
}

func (c *VerificationController) Run(ctx context.Context, req VerificationRequest, emit func(VerificationProgress)) (verifyfs.Report, error) {
	if c == nil || c.devices == nil || c.engine == nil {
		return verifyfs.Report{}, errors.New("verification controller is not configured")
	}
	if req.DeviceID == "" || req.MountPoint == "" {
		return verifyfs.Report{}, errors.New("device id and mount point are required")
	}

	devices, err := c.devices.List(ctx)
	if err != nil {
		return verifyfs.Report{}, fmt.Errorf("refresh devices: %w", err)
	}

	var selected *DeviceCard
	for _, d := range devices {
		if device.ID(d) == req.DeviceID {
			card := mapDevice(d)
			selected = &card
			break
		}
	}
	if selected == nil {
		return verifyfs.Report{}, ErrDeviceNotFound
	}
	if !slices.Contains(selected.MountPoints, req.MountPoint) {
		return verifyfs.Report{}, ErrMountNotFound
	}
	if selected.SystemDisk || selected.ReadOnly || !selected.LikelyExternal {
		return verifyfs.Report{}, ErrTargetProtected
	}
	if !slices.Contains(selected.WritableMountPoints, req.MountPoint) {
		return verifyfs.Report{}, ErrMountNotWritable
	}

	seed, err := c.seed()
	if err != nil {
		return verifyfs.Report{}, fmt.Errorf("create verification seed: %w", err)
	}

	report, err := c.engine.Run(ctx, verifyfs.Config{
		Root:       req.MountPoint,
		TotalBytes: req.TotalBytes,
		ChunkBytes: req.ChunkBytes,
		Seed:       seed,
	}, func(progress verifyfs.Progress) {
		if emit != nil {
			emit(VerificationProgress(progress))
		}
	})
	if err != nil {
		return report, fmt.Errorf("verify filesystem: %w", err)
	}
	return report, nil
}

func randomSeed() ([32]byte, error) {
	var seed [32]byte
	_, err := rand.Read(seed[:])
	return seed, err
}

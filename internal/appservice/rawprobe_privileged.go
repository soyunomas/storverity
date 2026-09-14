package appservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/soyunomas/storverity/internal/privhelper"
	"github.com/soyunomas/storverity/internal/rawprobe"
)

type PrivilegedRawProbeClient interface {
	RunRawProbe(context.Context, privhelper.RunRequest, func(rawprobe.Progress)) (rawprobe.Report, error)
}

type PrivilegedRawProbeController struct {
	confirmation *RawProbeController
	client       PrivilegedRawProbeClient
	sessionID    func() (string, error)
}

func NewPrivilegedRawProbeController(devices DeviceSource, client PrivilegedRawProbeClient) *PrivilegedRawProbeController {
	return &PrivilegedRawProbeController{
		confirmation: newRawProbeController(devices),
		client:       client,
		sessionID:    privhelper.NewSessionID,
	}
}

func (c *PrivilegedRawProbeController) Prepare(ctx context.Context, deviceID string) (RawProbeChallenge, error) {
	if c == nil || c.confirmation == nil {
		return RawProbeChallenge{}, errors.New("privileged raw probe controller is not configured")
	}
	return c.confirmation.Prepare(ctx, deviceID)
}

func (c *PrivilegedRawProbeController) Run(ctx context.Context, req RawProbeRequest, emit func(RawProbeProgress)) (rawprobe.Report, error) {
	if c == nil || c.confirmation == nil || c.client == nil || c.sessionID == nil {
		return rawprobe.Report{}, errors.New("privileged raw probe controller is not configured")
	}
	_, record, err := c.confirmation.authorizeRun(ctx, req)
	if err != nil {
		return rawprobe.Report{}, err
	}
	sessionID, err := c.sessionID()
	if err != nil {
		return rawprobe.Report{}, fmt.Errorf("create privileged raw probe session: %w", err)
	}
	report, runErr := c.client.RunRawProbe(ctx, privhelper.RunRequest{
		SessionID:           sessionID,
		DeviceID:            req.DeviceID,
		ExpectedFingerprint: record.fingerprint,
		Samples:             req.Samples,
		BlockBytes:          req.BlockBytes,
	}, func(progress rawprobe.Progress) {
		if emit != nil {
			emit(progress)
		}
	})
	if runErr != nil {
		return report, fmt.Errorf("privileged raw capacity probe: %w", runErr)
	}
	return report, nil
}

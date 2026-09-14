package appservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/privhelper"
	"github.com/soyunomas/storverity/internal/rawprobe"
)

type fakePrivilegedClient struct {
	req      privhelper.RunRequest
	report   rawprobe.Report
	err      error
	progress []rawprobe.Progress
	calls    int
}

func (c *fakePrivilegedClient) RunRawProbe(_ context.Context, req privhelper.RunRequest, emit func(rawprobe.Progress)) (rawprobe.Report, error) {
	c.calls++
	c.req = req
	for _, progress := range c.progress {
		if emit != nil {
			emit(progress)
		}
	}
	return c.report, c.err
}

func configuredPrivilegedController(source DeviceSource, client PrivilegedRawProbeClient) *PrivilegedRawProbeController {
	controller := NewPrivilegedRawProbeController(source, client)
	controller.confirmation.token = func() (string, error) { return "challenge-token", nil }
	controller.confirmation.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	controller.sessionID = func() (string, error) { return "session_0123456789abcdef", nil }
	return controller
}

func TestPrivilegedControllerPassesConfirmedIdentityToHelper(t *testing.T) {
	dev := eligibleRawDevice()
	client := &fakePrivilegedClient{
		report:   rawprobe.Report{Samples: rawprobe.DefaultSamples, Restored: true},
		progress: []rawprobe.Progress{{Phase: rawprobe.PhaseWrite, Sample: 1, SamplesTotal: rawprobe.DefaultSamples, Outcome: rawprobe.OutcomeWritten}},
	}
	controller := configuredPrivilegedController(fakeSource{devices: []device.Device{dev}}, client)
	challenge, err := controller.Prepare(context.Background(), device.ID(dev))
	if err != nil {
		t.Fatal(err)
	}
	var progress []RawProbeProgress
	report, err := controller.Run(context.Background(), RawProbeRequest{
		DeviceID:       device.ID(dev),
		ChallengeToken: challenge.Token,
		Confirmation:   challenge.ConfirmationText,
		Samples:        8,
		BlockBytes:     512,
	}, func(item RawProbeProgress) { progress = append(progress, item) })
	if err != nil {
		t.Fatal(err)
	}
	if report.Samples != rawprobe.DefaultSamples || client.calls != 1 || len(progress) != 1 {
		t.Fatalf("report=%+v calls=%d progress=%d", report, client.calls, len(progress))
	}
	if client.req.SessionID != "session_0123456789abcdef" || client.req.DeviceID != device.ID(dev) {
		t.Fatalf("request=%+v", client.req)
	}
	if client.req.ExpectedFingerprint != device.Fingerprint(dev) || client.req.Samples != 8 || client.req.BlockBytes != 512 {
		t.Fatalf("request=%+v", client.req)
	}
}

func TestPrivilegedControllerRejectsIdentityChangeBeforePolkit(t *testing.T) {
	before := eligibleRawDevice()
	after := before
	after.MajorMinor = "8:32"
	client := &fakePrivilegedClient{}
	controller := configuredPrivilegedController(&sequenceSource{lists: [][]device.Device{{before}, {after}}}, client)
	challenge, err := controller.Prepare(context.Background(), device.ID(before))
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Run(context.Background(), RawProbeRequest{
		DeviceID:       device.ID(before),
		ChallengeToken: challenge.Token,
		Confirmation:   challenge.ConfirmationText,
	}, nil)
	if !errors.Is(err, ErrRawProbeIdentity) {
		t.Fatalf("err=%v", err)
	}
	if client.calls != 0 {
		t.Fatalf("helper called %d times for stale identity", client.calls)
	}
}

func TestPrivilegedControllerPreservesCancellation(t *testing.T) {
	dev := eligibleRawDevice()
	client := &fakePrivilegedClient{err: context.Canceled}
	controller := configuredPrivilegedController(fakeSource{devices: []device.Device{dev}}, client)
	challenge, err := controller.Prepare(context.Background(), device.ID(dev))
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Run(context.Background(), RawProbeRequest{
		DeviceID:       device.ID(dev),
		ChallengeToken: challenge.Token,
		Confirmation:   challenge.ConfirmationText,
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

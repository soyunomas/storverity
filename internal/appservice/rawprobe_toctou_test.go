package appservice

import (
	"context"
	"errors"
	"testing"

	"github.com/soyunomas/storverity/internal/device"
)

func TestRawProbeRejectsIdentityChangeAfterOpenBeforeWrites(t *testing.T) {
	before := eligibleRawDevice()
	afterOpen := before
	afterOpen.MajorMinor = "8:32"
	source := &sequenceSource{lists: [][]device.Device{{before}, {before}, {afterOpen}}}
	engine := &fakeRawEngine{}
	opener := &fakeRawOpener{}
	controller := configuredRawController(source, engine, opener)

	challenge, err := controller.Prepare(context.Background(), "serial:USB123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Run(context.Background(), RawProbeRequest{
		DeviceID: "serial:USB123", ChallengeToken: challenge.Token, Confirmation: challenge.ConfirmationText,
	}, nil)
	if !errors.Is(err, ErrRawProbeIdentity) {
		t.Fatalf("err=%v", err)
	}
	if opener.calls != 1 {
		t.Fatalf("open calls=%d want=1", opener.calls)
	}
	if engine.calls != 0 {
		t.Fatalf("engine ran %d times after post-open identity change", engine.calls)
	}
}

func TestRawProbeRejectsMountAppearingAfterOpenBeforeWrites(t *testing.T) {
	before := eligibleRawDevice()
	afterOpen := before
	afterOpen.MountPoints = []string{"/media/USB"}
	source := &sequenceSource{lists: [][]device.Device{{before}, {before}, {afterOpen}}}
	engine := &fakeRawEngine{}
	controller := configuredRawController(source, engine, &fakeRawOpener{})

	challenge, err := controller.Prepare(context.Background(), "serial:USB123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Run(context.Background(), RawProbeRequest{
		DeviceID: "serial:USB123", ChallengeToken: challenge.Token, Confirmation: challenge.ConfirmationText,
	}, nil)
	if !errors.Is(err, ErrRawProbeProtected) {
		t.Fatalf("err=%v", err)
	}
	if engine.calls != 0 {
		t.Fatalf("engine ran %d times after post-open mount appeared", engine.calls)
	}
}

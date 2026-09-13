package appservice

import (
	"context"
	"errors"
	"time"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/verifyfs"
)

const defaultListTimeout = 5 * time.Second

// ProgressSink delivers structured verification progress to the desktop shell.
// The application service itself does not depend on Wails; the shell injects
// an event sink when it constructs Desktop.
type ProgressSink func(VerificationProgress)

// Desktop is the UI-facing application service bound by Wails. It intentionally
// exposes application operations only; low-level device, safety, and verifier
// packages are never bound directly to JavaScript.
type Desktop struct {
	devices      *Service
	verification *VerificationManager
	emitProgress ProgressSink
	listTimeout  time.Duration
}

func NewDesktop(source DeviceSource, engine FilesystemVerifier, emit ProgressSink) *Desktop {
	controller := NewVerificationController(source, engine)
	return &Desktop{
		devices:      New(source),
		verification: NewVerificationManager(controller),
		emitProgress: emit,
		listTimeout:  defaultListTimeout,
	}
}

func NewLinuxDesktop(emit ProgressSink) *Desktop {
	source := device.NewScanner()
	return NewDesktop(source, verifyfs.New(), emit)
}

// ListDevices refreshes discovery for every request so device cards do not
// silently outlive removable-media state.
func (d *Desktop) ListDevices() ([]DeviceCard, error) {
	if d == nil || d.devices == nil {
		return nil, errors.New("desktop service is not configured")
	}
	timeout := d.listTimeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return d.devices.ListDevices(ctx)
}

// StartVerification runs the non-destructive filesystem verifier. Wails calls
// bound Go methods asynchronously from JavaScript, while progress is delivered
// separately through the injected event sink.
func (d *Desktop) StartVerification(req VerificationRequest) (verifyfs.Report, error) {
	if d == nil || d.verification == nil {
		return verifyfs.Report{}, errors.New("desktop service is not configured")
	}
	return d.verification.Run(context.Background(), req, func(progress VerificationProgress) {
		if d.emitProgress != nil {
			d.emitProgress(progress)
		}
	})
}

func (d *Desktop) CancelVerification() bool {
	return d != nil && d.verification != nil && d.verification.Cancel()
}

func (d *Desktop) VerificationActive() bool {
	return d != nil && d.verification != nil && d.verification.Active()
}

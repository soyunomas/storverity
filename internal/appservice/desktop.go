package appservice

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/rawprobe"
	"github.com/soyunomas/storverity/internal/verifyfs"
)

const defaultListTimeout = 5 * time.Second

var ErrStorageOperationRunning = errors.New("another storage operation is already running")

// ProgressSink delivers structured verification progress to the desktop shell.
type ProgressSink func(VerificationProgress)

type RawProgressSink func(RawProbeProgress)

// Desktop is the UI-facing application service bound by Wails. It intentionally
// exposes application operations only; low-level device, safety, and verifier
// packages are never bound directly to JavaScript.
type Desktop struct {
	devices      *Service
	verification *VerificationManager
	rawControl   *RawProbeController
	rawProbe     *RawProbeManager
	reports      *ReportManager
	emitProgress ProgressSink
	emitRaw      RawProgressSink
	listTimeout  time.Duration

	operationMu sync.Mutex
	operation   string
}

func NewDesktop(source DeviceSource, engine FilesystemVerifier, emit ProgressSink) *Desktop {
	controller := NewVerificationController(source, engine)
	return &Desktop{
		devices:      New(source),
		verification: NewVerificationManager(controller),
		reports:      NewReportManager(nil),
		emitProgress: emit,
		listTimeout:  defaultListTimeout,
	}
}

func NewLinuxDesktop(emit ProgressSink, emitRaw RawProgressSink) *Desktop {
	return NewLinuxDesktopWithReportSaver(emit, emitRaw, nil)
}

func NewLinuxDesktopWithReportSaver(emit ProgressSink, emitRaw RawProgressSink, saver ReportSaver) *Desktop {
	source := device.NewScanner()
	desktop := NewDesktop(source, verifyfs.New(), emit)
	rawControl := NewRawProbeController(source, rawprobe.New(), RawMediaOpenFunc(rawprobe.OpenLinuxBlockDevice))
	desktop.rawControl = rawControl
	desktop.rawProbe = NewRawProbeManager(rawControl)
	desktop.reports = NewReportManager(saver)
	desktop.emitRaw = emitRaw
	return desktop
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

// StartVerification runs the non-destructive filesystem verifier and records a
// report for both successful and failed/cancelled runs that entered execution.
func (d *Desktop) StartVerification(req VerificationRequest) (verifyfs.Report, error) {
	if d == nil || d.verification == nil {
		return verifyfs.Report{}, errors.New("desktop service is not configured")
	}
	if err := d.beginOperation("filesystem"); err != nil {
		return verifyfs.Report{}, err
	}
	defer d.endOperation("filesystem")

	card := d.snapshotReportDevice(req.DeviceID)
	started := time.Now().UTC()
	result, runErr := d.verification.Run(context.Background(), req, func(progress VerificationProgress) {
		if d.emitProgress != nil {
			d.emitProgress(progress)
		}
	})
	d.recordFilesystemReport(card, req, result, runErr, started, time.Now().UTC())
	return result, runErr
}

func (d *Desktop) CancelVerification() bool {
	return d != nil && d.verification != nil && d.verification.Cancel()
}

func (d *Desktop) VerificationActive() bool {
	return d != nil && d.verification != nil && d.verification.Active()
}

func (d *Desktop) PrepareRawProbe(deviceID string) (RawProbeChallenge, error) {
	if d == nil || d.rawControl == nil {
		return RawProbeChallenge{}, errors.New("raw probe service is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultListTimeout)
	defer cancel()
	return d.rawControl.Prepare(ctx, deviceID)
}

// StartRawProbe runs the destructive sampled probe and records the restoration
// and fake-capacity result even when the operation returns an error.
func (d *Desktop) StartRawProbe(req RawProbeRequest) (rawprobe.Report, error) {
	if d == nil || d.rawProbe == nil {
		return rawprobe.Report{}, errors.New("raw probe service is not configured")
	}
	if err := d.beginOperation("raw"); err != nil {
		return rawprobe.Report{}, err
	}
	defer d.endOperation("raw")

	card := d.snapshotReportDevice(req.DeviceID)
	started := time.Now().UTC()
	result, runErr := d.rawProbe.Run(context.Background(), req, func(progress RawProbeProgress) {
		if d.emitRaw != nil {
			d.emitRaw(progress)
		}
	})
	d.recordRawReport(card, req, result, runErr, started, time.Now().UTC())
	return result, runErr
}

func (d *Desktop) CancelRawProbe() bool {
	return d != nil && d.rawProbe != nil && d.rawProbe.Cancel()
}

func (d *Desktop) RawProbeActive() bool {
	return d != nil && d.rawProbe != nil && d.rawProbe.Active()
}

func (d *Desktop) beginOperation(kind string) error {
	d.operationMu.Lock()
	defer d.operationMu.Unlock()
	if d.operation != "" {
		return ErrStorageOperationRunning
	}
	d.operation = kind
	return nil
}

func (d *Desktop) endOperation(kind string) {
	d.operationMu.Lock()
	if d.operation == kind {
		d.operation = ""
	}
	d.operationMu.Unlock()
}

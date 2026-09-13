package appservice

import (
	"context"
	"errors"
	"testing"

	"github.com/soyunomas/storverity/internal/rawprobe"
)

type blockingRawRunner struct {
	started chan struct{}
}

func (r *blockingRawRunner) Run(ctx context.Context, _ RawProbeRequest, _ func(RawProbeProgress)) (rawprobe.Report, error) {
	close(r.started)
	<-ctx.Done()
	return rawprobe.Report{Restored: true}, ctx.Err()
}

func TestRawProbeManagerRejectsDuplicateRunAndCancelsActiveRun(t *testing.T) {
	runner := &blockingRawRunner{started: make(chan struct{})}
	manager := NewRawProbeManager(runner)
	result := make(chan error, 1)
	go func() {
		_, err := manager.Run(context.Background(), RawProbeRequest{}, nil)
		result <- err
	}()
	<-runner.started

	if !manager.Active() {
		t.Fatal("Active() = false while probe is running")
	}
	if _, err := manager.Run(context.Background(), RawProbeRequest{}, nil); !errors.Is(err, ErrRawProbeRunning) {
		t.Fatalf("duplicate run err=%v", err)
	}
	if !manager.Cancel() {
		t.Fatal("Cancel() = false for active probe")
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled run err=%v", err)
	}
	if manager.Active() {
		t.Fatal("Active() = true after cancellation completed")
	}
	if manager.Cancel() {
		t.Fatal("Cancel() = true with no active probe")
	}
}

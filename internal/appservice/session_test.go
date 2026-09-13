package appservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/soyunomas/storverity/internal/verifyfs"
)

type blockingRunner struct {
	started chan struct{}
}

func (r *blockingRunner) Run(ctx context.Context, _ VerificationRequest, _ func(VerificationProgress)) (verifyfs.Report, error) {
	close(r.started)
	<-ctx.Done()
	return verifyfs.Report{}, ctx.Err()
}

type immediateRunner struct {
	report verifyfs.Report
	err    error
}

func (r immediateRunner) Run(context.Context, VerificationRequest, func(VerificationProgress)) (verifyfs.Report, error) {
	return r.report, r.err
}

func TestVerificationManagerCancelStopsActiveRun(t *testing.T) {
	runner := &blockingRunner{started: make(chan struct{})}
	manager := NewVerificationManager(runner)
	result := make(chan error, 1)

	go func() {
		_, err := manager.Run(context.Background(), VerificationRequest{}, nil)
		result <- err
	}()

	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("verification did not start")
	}

	if !manager.Active() {
		t.Fatal("Active() = false while runner is blocked")
	}
	if !manager.Cancel() {
		t.Fatal("Cancel() = false, want true")
	}

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("verification did not stop after cancellation")
	}

	if manager.Active() {
		t.Fatal("Active() = true after Run returned")
	}
	if manager.Cancel() {
		t.Fatal("Cancel() = true without an active run")
	}
}

func TestVerificationManagerRejectsConcurrentRun(t *testing.T) {
	runner := &blockingRunner{started: make(chan struct{})}
	manager := NewVerificationManager(runner)
	done := make(chan struct{})

	go func() {
		defer close(done)
		_, _ = manager.Run(context.Background(), VerificationRequest{}, nil)
	}()
	<-runner.started

	if _, err := manager.Run(context.Background(), VerificationRequest{}, nil); !errors.Is(err, ErrVerificationRunning) {
		t.Fatalf("second Run() error = %v, want ErrVerificationRunning", err)
	}
	manager.Cancel()
	<-done
}

func TestVerificationManagerClearsStateAfterCompletion(t *testing.T) {
	want := verifyfs.Report{BytesWritten: 10, BytesVerified: 10, Regions: 1}
	manager := NewVerificationManager(immediateRunner{report: want})

	got, err := manager.Run(context.Background(), VerificationRequest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("report = %+v, want %+v", got, want)
	}
	if manager.Active() {
		t.Fatal("Active() = true after successful completion")
	}
}

func TestVerificationManagerRejectsMissingRunner(t *testing.T) {
	if _, err := NewVerificationManager(nil).Run(context.Background(), VerificationRequest{}, nil); err == nil {
		t.Fatal("Run() error = nil, want configuration error")
	}
}

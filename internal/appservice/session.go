package appservice

import (
	"context"
	"errors"
	"sync"

	"github.com/soyunomas/storverity/internal/verifyfs"
)

var ErrVerificationRunning = errors.New("a verification is already running")

// VerificationRunner is the UI-independent operation exposed to the session
// manager. VerificationController satisfies this interface.
type VerificationRunner interface {
	Run(context.Context, VerificationRequest, func(VerificationProgress)) (verifyfs.Report, error)
}

// VerificationManager serializes filesystem verification runs and owns the
// cancellation function used by the desktop application's Stop action.
//
// StorVerity intentionally permits only one verification at a time. Besides
// keeping the UI state simple, this avoids accidental concurrent writes to two
// mount points when a user double-clicks or sends duplicate requests.
type VerificationManager struct {
	mu     sync.Mutex
	runner VerificationRunner
	nextID uint64
	active uint64
	cancel context.CancelFunc
}

func NewVerificationManager(runner VerificationRunner) *VerificationManager {
	return &VerificationManager{runner: runner}
}

func (m *VerificationManager) Run(ctx context.Context, req VerificationRequest, emit func(VerificationProgress)) (verifyfs.Report, error) {
	if m == nil || m.runner == nil {
		return verifyfs.Report{}, errors.New("verification manager is not configured")
	}

	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return verifyfs.Report{}, ErrVerificationRunning
	}

	runCtx, cancel := context.WithCancel(ctx)
	m.nextID++
	runID := m.nextID
	m.active = runID
	m.cancel = cancel
	m.mu.Unlock()

	defer func() {
		cancel()
		m.mu.Lock()
		if m.active == runID {
			m.active = 0
			m.cancel = nil
		}
		m.mu.Unlock()
	}()

	return m.runner.Run(runCtx, req, emit)
}

// Cancel requests cancellation of the active verification. It returns false
// when there is no active run. Completion is asynchronous: callers should keep
// showing a stopping state until Run returns.
func (m *VerificationManager) Cancel() bool {
	if m == nil {
		return false
	}

	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (m *VerificationManager) Active() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancel != nil
}

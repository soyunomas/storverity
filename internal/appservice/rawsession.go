package appservice

import (
	"context"
	"errors"
	"sync"

	"github.com/soyunomas/storverity/internal/rawprobe"
)

var ErrRawProbeRunning = errors.New("a raw capacity probe is already running")

type RawProbeRunner interface {
	Run(context.Context, RawProbeRequest, func(RawProbeProgress)) (rawprobe.Report, error)
}

type RawProbeManager struct {
	mu     sync.Mutex
	runner RawProbeRunner
	nextID uint64
	active uint64
	cancel context.CancelFunc
}

func NewRawProbeManager(runner RawProbeRunner) *RawProbeManager {
	return &RawProbeManager{runner: runner}
}

func (m *RawProbeManager) Run(ctx context.Context, req RawProbeRequest, emit func(RawProbeProgress)) (rawprobe.Report, error) {
	if m == nil || m.runner == nil {
		return rawprobe.Report{}, errors.New("raw probe manager is not configured")
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return rawprobe.Report{}, ErrRawProbeRunning
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

func (m *RawProbeManager) Cancel() bool {
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

func (m *RawProbeManager) Active() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancel != nil
}

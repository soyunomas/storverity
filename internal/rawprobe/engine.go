package rawprobe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Phase string

type Outcome string

const (
	PhaseSnapshot Phase = "snapshot"
	PhaseWrite    Phase = "write"
	PhaseVerify   Phase = "verify"
	PhaseRestore  Phase = "restore"

	OutcomeSnapshot    Outcome = "snapshot"
	OutcomeWritten     Outcome = "written"
	OutcomeValid       Outcome = "valid"
	OutcomeCorrupt     Outcome = "corrupt"
	OutcomeReadError   Outcome = "read-error"
	OutcomeWriteError  Outcome = "write-error"
	OutcomeRestoreError Outcome = "restore-error"
)

type Media interface {
	ReadAt([]byte, int64) (int, error)
	WriteAt([]byte, int64) (int, error)
	Sync() error
	Close() error
}

type Config struct {
	CapacityBytes uint64
	Samples       int
	BlockBytes    uint64
	GuardBytes    uint64
	Seed          [32]byte
}

type Progress struct {
	Phase        Phase   `json:"phase"`
	Sample       int     `json:"sample"`
	SamplesTotal int     `json:"samplesTotal"`
	OffsetBytes  uint64  `json:"offsetBytes"`
	Outcome      Outcome `json:"outcome"`
	Error        string  `json:"error,omitempty"`
}

type SampleResult struct {
	Index        int     `json:"index"`
	OffsetBytes  uint64  `json:"offsetBytes"`
	Outcome      Outcome `json:"outcome"`
	Error        string  `json:"error,omitempty"`
	Restored     bool    `json:"restored"`
	RestoreError string  `json:"restoreError,omitempty"`
}

type Report struct {
	AdvertisedBytes       uint64         `json:"advertisedBytes"`
	BlockBytes            uint64         `json:"blockBytes"`
	Samples               int            `json:"samples"`
	ValidSamples          int            `json:"validSamples"`
	CorruptSamples        int            `json:"corruptSamples"`
	ReadErrors            int            `json:"readErrors"`
	WriteErrors           int            `json:"writeErrors"`
	RestoreErrors         int            `json:"restoreErrors"`
	ValidatedThroughBytes uint64         `json:"validatedThroughBytes"`
	SuspectFakeCapacity   bool           `json:"suspectFakeCapacity"`
	Restored              bool           `json:"restored"`
	Results               []SampleResult `json:"results"`
}

type Engine struct{}

func New() *Engine { return &Engine{} }

type sampleState struct {
	sample   Sample
	original []byte
	touched  bool
	result   SampleResult
}

func (e *Engine) Run(ctx context.Context, media Media, cfg Config, emit func(Progress)) (Report, error) {
	if media == nil {
		return Report{}, errors.New("raw media is not configured")
	}
	if cfg.Samples == 0 {
		cfg.Samples = DefaultSamples
	}
	if cfg.BlockBytes == 0 {
		cfg.BlockBytes = DefaultBlockBytes
	}
	if cfg.GuardBytes == 0 {
		cfg.GuardBytes = DefaultGuardBytes
	}
	plan, err := Plan(cfg.CapacityBytes, cfg.Samples, cfg.BlockBytes, cfg.GuardBytes)
	if err != nil {
		return Report{}, err
	}
	states := make([]sampleState, len(plan))
	for i, sample := range plan {
		states[i].sample = sample
		states[i].result = SampleResult{Index: sample.Index, OffsetBytes: sample.Offset}
	}

	emitProgress := func(phase Phase, st *sampleState, outcome Outcome, err error) {
		if emit == nil {
			return
		}
		p := Progress{Phase: phase, Sample: st.sample.Index, SamplesTotal: len(states), OffsetBytes: st.sample.Offset, Outcome: outcome}
		if err != nil {
			p.Error = err.Error()
		}
		emit(p)
	}

	var runErr error
	for i := range states {
		if err := ctx.Err(); err != nil {
			runErr = err
			break
		}
		st := &states[i]
		st.original = make([]byte, cfg.BlockBytes)
		if err := readFullAt(media, st.original, st.sample.Offset); err != nil {
			st.result.Outcome = OutcomeReadError
			st.result.Error = err.Error()
			emitProgress(PhaseSnapshot, st, OutcomeReadError, err)
			continue
		}
		st.result.Outcome = OutcomeSnapshot
		emitProgress(PhaseSnapshot, st, OutcomeSnapshot, nil)
	}

	if runErr == nil {
		for i := range states {
			if err := ctx.Err(); err != nil {
				runErr = err
				break
			}
			st := &states[i]
			if st.result.Outcome == OutcomeReadError {
				continue
			}
			pattern := makePattern(cfg.Seed, st.sample, int(cfg.BlockBytes))
			n, err := media.WriteAt(pattern, int64(st.sample.Offset))
			if n > 0 {
				st.touched = true
			}
			if err != nil || n != len(pattern) {
				if err == nil {
					err = io.ErrShortWrite
				}
				st.result.Outcome = OutcomeWriteError
				st.result.Error = err.Error()
				emitProgress(PhaseWrite, st, OutcomeWriteError, err)
				continue
			}
			st.touched = true
			st.result.Outcome = OutcomeWritten
			emitProgress(PhaseWrite, st, OutcomeWritten, nil)
		}
		if err := media.Sync(); err != nil {
			runErr = fmt.Errorf("flush raw probe writes: %w", err)
		}
	}

	if runErr == nil {
		for i := len(states) - 1; i >= 0; i-- {
			if err := ctx.Err(); err != nil {
				runErr = err
				break
			}
			st := &states[i]
			if st.result.Outcome != OutcomeWritten {
				continue
			}
			buf := make([]byte, cfg.BlockBytes)
			if err := readFullAt(media, buf, st.sample.Offset); err != nil {
				st.result.Outcome = OutcomeReadError
				st.result.Error = err.Error()
				emitProgress(PhaseVerify, st, OutcomeReadError, err)
				continue
			}
			want := makePattern(cfg.Seed, st.sample, int(cfg.BlockBytes))
			if !bytes.Equal(buf, want) {
				err := errors.New("verification pattern mismatch")
				st.result.Outcome = OutcomeCorrupt
				st.result.Error = err.Error()
				emitProgress(PhaseVerify, st, OutcomeCorrupt, err)
				continue
			}
			st.result.Outcome = OutcomeValid
			st.result.Error = ""
			emitProgress(PhaseVerify, st, OutcomeValid, nil)
		}
	}

	restoreErr := restore(states, media, emitProgress)
	if runErr == nil && restoreErr != nil {
		runErr = restoreErr
	}
	report := buildReport(cfg, states)
	return report, runErr
}

func restore(states []sampleState, media Media, emit func(Phase, *sampleState, Outcome, error)) error {
	var firstErr error
	for i := len(states) - 1; i >= 0; i-- {
		st := &states[i]
		if !st.touched || len(st.original) == 0 {
			continue
		}
		n, err := media.WriteAt(st.original, int64(st.sample.Offset))
		if err != nil || n != len(st.original) {
			if err == nil {
				err = io.ErrShortWrite
			}
			st.result.RestoreError = err.Error()
			emit(PhaseRestore, st, OutcomeRestoreError, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		st.result.Restored = true
		emit(PhaseRestore, st, st.result.Outcome, nil)
	}
	if err := media.Sync(); err != nil && firstErr == nil {
		firstErr = err
	}
	if firstErr != nil {
		return fmt.Errorf("restore sampled raw bytes: %w", firstErr)
	}
	return nil
}

func buildReport(cfg Config, states []sampleState) Report {
	report := Report{
		AdvertisedBytes: cfg.CapacityBytes,
		BlockBytes:      cfg.BlockBytes,
		Samples:         len(states),
		Restored:        true,
		Results:         make([]SampleResult, len(states)),
	}
	contiguousValid := true
	for i := range states {
		result := states[i].result
		report.Results[i] = result
		switch result.Outcome {
		case OutcomeValid:
			report.ValidSamples++
		case OutcomeCorrupt:
			report.CorruptSamples++
		case OutcomeReadError:
			report.ReadErrors++
		case OutcomeWriteError:
			report.WriteErrors++
		}
		if result.RestoreError != "" {
			report.RestoreErrors++
			report.Restored = false
		}
		if contiguousValid && result.Outcome == OutcomeValid {
			report.ValidatedThroughBytes = result.OffsetBytes + cfg.BlockBytes
		} else if result.Outcome != OutcomeValid {
			contiguousValid = false
		}
	}
	report.SuspectFakeCapacity = report.CorruptSamples > 0 || report.ReadErrors > 0 || report.WriteErrors > 0
	return report
}

func readFullAt(media Media, buf []byte, offset uint64) error {
	n, err := media.ReadAt(buf, int64(offset))
	if err != nil {
		return err
	}
	if n != len(buf) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func makePattern(seed [32]byte, sample Sample, size int) []byte {
	out := make([]byte, size)
	var meta [24]byte
	binary.LittleEndian.PutUint64(meta[0:8], uint64(sample.Index))
	binary.LittleEndian.PutUint64(meta[8:16], sample.Offset)
	var counter uint64
	for pos := 0; pos < len(out); {
		binary.LittleEndian.PutUint64(meta[16:24], counter)
		h := sha256.New()
		_, _ = h.Write(seed[:])
		_, _ = h.Write(meta[:])
		chunk := h.Sum(nil)
		pos += copy(out[pos:], chunk)
		counter++
	}
	return out
}

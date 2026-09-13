package verifyfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var ErrCorrupt = errors.New("verification data mismatch")

type Phase string

const (
	PhaseWrite  Phase = "write"
	PhaseVerify Phase = "verify"
)

type Config struct {
	Root       string
	TotalBytes int64
	ChunkBytes int64
	Seed       [32]byte
}

type Progress struct {
	Phase          Phase `json:"phase"`
	Region         int   `json:"region"`
	RegionsTotal   int   `json:"regionsTotal"`
	BytesCompleted int64 `json:"bytesCompleted"`
	BytesTotal     int64 `json:"bytesTotal"`
}

type Report struct {
	BytesWritten  int64 `json:"bytesWritten"`
	BytesVerified int64 `json:"bytesVerified"`
	Regions       int   `json:"regions"`
}

type Engine struct {
	afterWrite func(testDir string) error
}

func New() *Engine { return &Engine{} }

func (e *Engine) Run(ctx context.Context, cfg Config, progress func(Progress)) (report Report, err error) {
	if err := validateConfig(cfg); err != nil {
		return Report{}, err
	}

	testDir, err := os.MkdirTemp(cfg.Root, ".storverity-")
	if err != nil {
		return Report{}, fmt.Errorf("create test directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(testDir); cleanupErr != nil && err == nil {
			err = fmt.Errorf("cleanup test directory: %w", cleanupErr)
		}
	}()

	regions := regionCount(cfg.TotalBytes, cfg.ChunkBytes)
	report.Regions = regions

	for region := 0; region < regions; region++ {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		size := regionSize(cfg.TotalBytes, cfg.ChunkBytes, region)
		path := filepath.Join(testDir, fmt.Sprintf("region-%06d.bin", region))
		if err := writeRegion(path, cfg.Seed, region, size); err != nil {
			return report, err
		}
		report.BytesWritten += size
		emit(progress, Progress{Phase: PhaseWrite, Region: region, RegionsTotal: regions, BytesCompleted: report.BytesWritten, BytesTotal: cfg.TotalBytes})
	}

	if e.afterWrite != nil {
		if err := e.afterWrite(testDir); err != nil {
			return report, fmt.Errorf("after-write hook: %w", err)
		}
	}

	for region := 0; region < regions; region++ {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		size := regionSize(cfg.TotalBytes, cfg.ChunkBytes, region)
		path := filepath.Join(testDir, fmt.Sprintf("region-%06d.bin", region))
		if err := verifyRegion(path, cfg.Seed, region, size); err != nil {
			return report, err
		}
		report.BytesVerified += size
		emit(progress, Progress{Phase: PhaseVerify, Region: region, RegionsTotal: regions, BytesCompleted: report.BytesVerified, BytesTotal: cfg.TotalBytes})
	}

	return report, nil
}

func validateConfig(cfg Config) error {
	if cfg.Root == "" {
		return errors.New("root directory is required")
	}
	info, err := os.Stat(cfg.Root)
	if err != nil {
		return fmt.Errorf("stat root directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("root path is not a directory")
	}
	if cfg.TotalBytes <= 0 {
		return errors.New("total bytes must be greater than zero")
	}
	if cfg.ChunkBytes <= 0 {
		return errors.New("chunk bytes must be greater than zero")
	}
	if cfg.ChunkBytes > 64<<20 {
		return errors.New("chunk bytes must not exceed 64 MiB")
	}
	return nil
}

func regionCount(total, chunk int64) int {
	return int((total + chunk - 1) / chunk)
}

func regionSize(total, chunk int64, region int) int64 {
	start := int64(region) * chunk
	remaining := total - start
	if remaining < chunk {
		return remaining
	}
	return chunk
}

func writeRegion(path string, seed [32]byte, region int, size int64) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create region %d: %w", region, err)
	}

	if _, err := io.CopyN(f, newPatternReader(seed, region), size); err != nil {
		_ = f.Close()
		return fmt.Errorf("write region %d: %w", region, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync region %d: %w", region, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close region %d: %w", region, err)
	}
	return nil
}

func verifyRegion(path string, seed [32]byte, region int, size int64) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open region %d: %w", region, err)
	}
	defer f.Close()

	expected := newPatternReader(seed, region)
	actual := make([]byte, 64*1024)
	wanted := make([]byte, len(actual))
	remaining := size

	for remaining > 0 {
		n := int64(len(actual))
		if remaining < n {
			n = remaining
		}
		if _, err := io.ReadFull(f, actual[:n]); err != nil {
			return fmt.Errorf("read region %d: %w", region, err)
		}
		if _, err := io.ReadFull(expected, wanted[:n]); err != nil {
			return fmt.Errorf("generate expected region %d: %w", region, err)
		}
		if !bytes.Equal(actual[:n], wanted[:n]) {
			return fmt.Errorf("region %d: %w", region, ErrCorrupt)
		}
		remaining -= n
	}

	extra := []byte{0}
	if n, err := f.Read(extra); err != io.EOF || n != 0 {
		if err != nil {
			return fmt.Errorf("check region %d length: %w", region, err)
		}
		return fmt.Errorf("region %d: unexpected trailing data: %w", region, ErrCorrupt)
	}
	return nil
}

func emit(fn func(Progress), p Progress) {
	if fn != nil {
		fn(p)
	}
}

type patternReader struct {
	seed    [32]byte
	region  uint64
	counter uint64
	block   [32]byte
	offset  int
}

func newPatternReader(seed [32]byte, region int) io.Reader {
	return &patternReader{seed: seed, region: uint64(region), offset: 32}
}

func (r *patternReader) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		if r.offset == len(r.block) {
			var input [48]byte
			copy(input[:32], r.seed[:])
			binary.LittleEndian.PutUint64(input[32:40], r.region)
			binary.LittleEndian.PutUint64(input[40:48], r.counter)
			r.block = sha256.Sum256(input[:])
			r.counter++
			r.offset = 0
		}
		n := copy(p[written:], r.block[r.offset:])
		r.offset += n
		written += n
	}
	return written, nil
}

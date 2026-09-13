package verifyfs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesVerifiesAndCleansUp(t *testing.T) {
	root := t.TempDir()
	var seed [32]byte
	copy(seed[:], []byte("storverity-test-seed"))

	var events []Progress
	report, err := New().Run(context.Background(), Config{
		Root: root, TotalBytes: 150_000, ChunkBytes: 64_000, Seed: seed,
	}, func(p Progress) { events = append(events, p) })
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.BytesWritten != 150_000 || report.BytesVerified != 150_000 || report.Regions != 3 {
		t.Fatalf("report = %+v", report)
	}
	if len(events) != 6 {
		t.Fatalf("events = %d, want 6", len(events))
	}
	for i := 0; i < 3; i++ {
		if events[i].Outcome != RegionWritten {
			t.Fatalf("events[%d].Outcome = %q, want %q", i, events[i].Outcome, RegionWritten)
		}
	}
	for i := 3; i < 6; i++ {
		if events[i].Outcome != RegionVerified {
			t.Fatalf("events[%d].Outcome = %q, want %q", i, events[i].Outcome, RegionVerified)
		}
	}
	assertNoTestDirs(t, root)
}

func TestRunDetectsCorruptionAndEmitsTypedFailure(t *testing.T) {
	root := t.TempDir()
	engine := New()
	engine.afterWrite = func(testDir string) error {
		path := filepath.Join(testDir, "region-000001.bin")
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err = f.WriteAt([]byte{0xFF, 0xEE, 0xDD, 0xCC}, 17); err != nil {
			return err
		}
		return f.Sync()
	}

	var events []Progress
	_, err := engine.Run(context.Background(), Config{Root: root, TotalBytes: 128_000, ChunkBytes: 64_000}, func(p Progress) {
		events = append(events, p)
	})
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Run() error = %v, want ErrCorrupt", err)
	}
	var regionErr *RegionError
	if !errors.As(err, &regionErr) || regionErr.Region != 1 || regionErr.Kind != FailureCorrupt {
		t.Fatalf("Run() region error = %+v, want region 1 corruption", regionErr)
	}
	last := events[len(events)-1]
	if last.Region != 1 || last.Outcome != RegionCorrupt || last.Error == "" {
		t.Fatalf("last progress = %+v, want typed corruption", last)
	}
	assertNoTestDirs(t, root)
}

func TestRunEmitsTypedWriteFailure(t *testing.T) {
	root := t.TempDir()
	engine := New()
	original := engine.write
	engine.write = func(path string, seed [32]byte, region int, size int64) error {
		if region == 1 {
			return errors.New("injected write failure")
		}
		return original(path, seed, region, size)
	}

	var events []Progress
	_, err := engine.Run(context.Background(), Config{Root: root, TotalBytes: 128_000, ChunkBytes: 64_000}, func(p Progress) {
		events = append(events, p)
	})
	var regionErr *RegionError
	if !errors.As(err, &regionErr) || regionErr.Region != 1 || regionErr.Kind != FailureWrite {
		t.Fatalf("Run() region error = %+v, want region 1 write failure", regionErr)
	}
	last := events[len(events)-1]
	if last.Region != 1 || last.Outcome != RegionWriteError || last.BytesCompleted != 64_000 || last.Error == "" {
		t.Fatalf("last progress = %+v, want typed write failure", last)
	}
	assertNoTestDirs(t, root)
}

func TestRunEmitsTypedReadFailure(t *testing.T) {
	root := t.TempDir()
	engine := New()
	original := engine.verify
	engine.verify = func(path string, seed [32]byte, region int, size int64) error {
		if region == 1 {
			return io.ErrUnexpectedEOF
		}
		return original(path, seed, region, size)
	}

	var events []Progress
	_, err := engine.Run(context.Background(), Config{Root: root, TotalBytes: 128_000, ChunkBytes: 64_000}, func(p Progress) {
		events = append(events, p)
	})
	var regionErr *RegionError
	if !errors.As(err, &regionErr) || regionErr.Region != 1 || regionErr.Kind != FailureRead {
		t.Fatalf("Run() region error = %+v, want region 1 read failure", regionErr)
	}
	last := events[len(events)-1]
	if last.Region != 1 || last.Outcome != RegionReadError || last.BytesCompleted != 64_000 || last.Error == "" {
		t.Fatalf("last progress = %+v, want typed read failure", last)
	}
	assertNoTestDirs(t, root)
}

func TestRunHonorsCancellationAndCleansUp(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	engine := New()
	engine.afterWrite = func(string) error {
		cancel()
		return nil
	}

	_, err := engine.Run(ctx, Config{Root: root, TotalBytes: 128_000, ChunkBytes: 64_000}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	assertNoTestDirs(t, root)
}

func TestRunValidatesConfig(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []Config{
		{},
		{Root: file, TotalBytes: 1, ChunkBytes: 1},
		{Root: root, TotalBytes: 0, ChunkBytes: 1},
		{Root: root, TotalBytes: 1, ChunkBytes: 0},
		{Root: root, TotalBytes: 1, ChunkBytes: (64 << 20) + 1},
	}
	for i, cfg := range tests {
		if _, err := New().Run(context.Background(), cfg, nil); err == nil {
			t.Fatalf("case %d: Run() error = nil", i)
		}
	}
}

func TestPatternDiffersByRegionAndIsRepeatable(t *testing.T) {
	var seed [32]byte
	copy(seed[:], []byte("same-seed"))
	a := readPattern(t, seed, 4, 128)
	b := readPattern(t, seed, 4, 128)
	c := readPattern(t, seed, 5, 128)
	if string(a) != string(b) {
		t.Fatal("same seed/region produced different data")
	}
	if string(a) == string(c) {
		t.Fatal("different regions produced identical data")
	}
}

func readPattern(t *testing.T, seed [32]byte, region, n int) []byte {
	t.Helper()
	out := make([]byte, n)
	if _, err := newPatternReader(seed, region).Read(out); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertNoTestDirs(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".storverity-") {
			t.Fatalf("temporary test directory left behind: %s", entry.Name())
		}
	}
}

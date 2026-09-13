package verifyfs

import (
	"context"
	"errors"
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
	assertNoTestDirs(t, root)
}

func TestRunDetectsCorruptionAndCleansUp(t *testing.T) {
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

	_, err := engine.Run(context.Background(), Config{Root: root, TotalBytes: 128_000, ChunkBytes: 64_000}, nil)
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Run() error = %v, want ErrCorrupt", err)
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

package rawprobe

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
)

type aliasFileMedia struct {
	file *os.File
	size int64
}

func (m *aliasFileMedia) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || m.size <= 0 {
		return 0, io.EOF
	}
	return m.file.ReadAt(p, off%m.size)
}

func (m *aliasFileMedia) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || m.size <= 0 {
		return 0, io.ErrShortWrite
	}
	return m.file.WriteAt(p, off%m.size)
}

func (m *aliasFileMedia) Sync() error  { return m.file.Sync() }
func (m *aliasFileMedia) Close() error { return m.file.Close() }

func createPatternFile(t *testing.T, size int) (*os.File, []byte) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "storverity-block-*")
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, size)
	for i := range data {
		data[i] = byte((i*29 + 13) % 251)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	return file, data
}

func readWholeFile(t *testing.T, file *os.File, size int) []byte {
	t.Helper()
	out := make([]byte, size)
	n, err := file.ReadAt(out, 0)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if n != size {
		t.Fatalf("read %d bytes, want %d", n, size)
	}
	return out
}

func TestFileBackedShortDeviceReportsReadErrorsWithoutChangingOriginal(t *testing.T) {
	const actual = 32 * 1024
	file, original := createPatternFile(t, actual)
	defer file.Close()

	report, err := New().Run(context.Background(), file, Config{
		CapacityBytes: 256 * 1024,
		Samples:       32,
		BlockBytes:    512,
		Seed:          [32]byte{9},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.ReadErrors == 0 || !report.SuspectFakeCapacity {
		t.Fatalf("expected short-device read errors, report=%+v", report)
	}
	if got := readWholeFile(t, file, actual); !bytes.Equal(got, original) {
		t.Fatal("short-device probe changed original file contents")
	}
}

func TestFileBackedAliasingReportsCorruptionAndRestoresPhysicalStorage(t *testing.T) {
	const actual = 32 * 1024
	file, original := createPatternFile(t, actual)
	media := &aliasFileMedia{file: file, size: actual}
	defer media.Close()

	report, err := New().Run(context.Background(), media, Config{
		CapacityBytes: 512 * 1024,
		Samples:       64,
		BlockBytes:    512,
		Seed:          [32]byte{10},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.CorruptSamples == 0 || !report.SuspectFakeCapacity {
		t.Fatalf("expected aliased capacity corruption, report=%+v", report)
	}
	if !report.Restored {
		t.Fatalf("expected best-effort restoration, report=%+v", report)
	}
	if got := readWholeFile(t, file, actual); !bytes.Equal(got, original) {
		t.Fatal("aliased file-backed probe did not restore physical storage")
	}
}

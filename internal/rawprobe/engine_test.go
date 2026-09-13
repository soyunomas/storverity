package rawprobe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type memoryMedia struct {
	data []byte
}

func newMemoryMedia(size int) *memoryMedia {
	m := &memoryMedia{data: make([]byte, size)}
	for i := range m.data {
		m.data[i] = byte((i*37 + 11) % 251)
	}
	return m
}

func (m *memoryMedia) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(p, m.data[int(off):])
	if n != len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (m *memoryMedia) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(m.data)) {
		return 0, io.ErrShortWrite
	}
	n := copy(m.data[int(off):], p)
	if n != len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}
func (m *memoryMedia) Sync() error  { return nil }
func (m *memoryMedia) Close() error { return nil }

type aliasMedia struct{ data []byte }

func newAliasMedia(actual int) *aliasMedia {
	m := &aliasMedia{data: make([]byte, actual)}
	for i := range m.data {
		m.data[i] = byte((i*19 + 7) % 251)
	}
	return m
}
func (m *aliasMedia) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || len(m.data) == 0 {
		return 0, io.EOF
	}
	for i := range p {
		p[i] = m.data[(int(off)+i)%len(m.data)]
	}
	return len(p), nil
}
func (m *aliasMedia) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || len(m.data) == 0 {
		return 0, io.ErrShortWrite
	}
	for i, b := range p {
		m.data[(int(off)+i)%len(m.data)] = b
	}
	return len(p), nil
}
func (m *aliasMedia) Sync() error  { return nil }
func (m *aliasMedia) Close() error { return nil }

type cancelMedia struct {
	*memoryMedia
	cancel context.CancelFunc
	writes int
}

func (m *cancelMedia) WriteAt(p []byte, off int64) (int, error) {
	n, err := m.memoryMedia.WriteAt(p, off)
	m.writes++
	if m.writes == 1 {
		m.cancel()
	}
	return n, err
}

func TestEngineValidMediaRestoresOriginalBytes(t *testing.T) {
	media := newMemoryMedia(256 * 1024)
	before := append([]byte(nil), media.data...)
	report, err := New().Run(context.Background(), media, Config{
		CapacityBytes: uint64(len(media.data)), Samples: 16, BlockBytes: 512, Seed: [32]byte{1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.ValidSamples != report.Samples || report.SuspectFakeCapacity || !report.Restored {
		t.Fatalf("report=%+v", report)
	}
	if !bytes.Equal(before, media.data) {
		t.Fatal("raw probe did not restore original bytes")
	}
}

func TestEngineDetectsAliasedFakeCapacity(t *testing.T) {
	media := newAliasMedia(32 * 1024)
	before := append([]byte(nil), media.data...)
	report, err := New().Run(context.Background(), media, Config{
		CapacityBytes: 512 * 1024, Samples: 64, BlockBytes: 512, Seed: [32]byte{2},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.SuspectFakeCapacity || report.CorruptSamples == 0 {
		t.Fatalf("expected alias corruption, report=%+v", report)
	}
	if !bytes.Equal(before, media.data) {
		t.Fatal("best-effort restoration changed aliased media")
	}
}

func TestEngineDetectsShortMediaAsReadErrors(t *testing.T) {
	media := newMemoryMedia(32 * 1024)
	report, err := New().Run(context.Background(), media, Config{
		CapacityBytes: 256 * 1024, Samples: 32, BlockBytes: 512, Seed: [32]byte{3},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.SuspectFakeCapacity || report.ReadErrors == 0 {
		t.Fatalf("expected read errors, report=%+v", report)
	}
}

func TestEngineCancellationRestoresTouchedSamples(t *testing.T) {
	base := newMemoryMedia(128 * 1024)
	before := append([]byte(nil), base.data...)
	ctx, cancel := context.WithCancel(context.Background())
	media := &cancelMedia{memoryMedia: base, cancel: cancel}
	report, err := New().Run(ctx, media, Config{
		CapacityBytes: uint64(len(base.data)), Samples: 16, BlockBytes: 512, Seed: [32]byte{4},
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if !report.Restored {
		t.Fatalf("report=%+v", report)
	}
	if !bytes.Equal(before, base.data) {
		t.Fatal("canceled probe did not restore touched sample")
	}
}

package rawprobe

import "testing"

func TestPlanRejectsUnsafeBlockGeometry(t *testing.T) {
	for _, block := range []uint64{0, 1, 511, 513, 1536, MaxBlockBytes + 1} {
		if _, err := Plan(16*1024*1024, 8, block, 0); err == nil {
			t.Fatalf("Plan accepted unsafe block size %d", block)
		}
	}
	if _, err := Plan(16*1024*1024, 8, MinBlockBytes, 0); err != nil {
		t.Fatalf("minimum block size rejected: %v", err)
	}
	if _, err := Plan(16*1024*1024, 8, MaxBlockBytes, 0); err != nil {
		t.Fatalf("maximum block size rejected: %v", err)
	}
}

package rawprobe

import "testing"

func TestPlanSpansAdvertisedCapacityAndIsAligned(t *testing.T) {
	const block = 4096
	plan, err := Plan(64*1024*1024, 8, block, 4*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 8 {
		t.Fatalf("samples=%d", len(plan))
	}
	for i, sample := range plan {
		if sample.Offset%block != 0 {
			t.Fatalf("offset %d is not aligned", sample.Offset)
		}
		if i > 0 && sample.Offset <= plan[i-1].Offset {
			t.Fatalf("offsets are not strictly increasing: %+v", plan)
		}
	}
	if plan[0].Offset < 4*1024*1024 {
		t.Fatalf("first offset=%d", plan[0].Offset)
	}
	if plan[len(plan)-1].Offset > 60*1024*1024 {
		t.Fatalf("last offset=%d", plan[len(plan)-1].Offset)
	}
}

func TestPlanFallsBackForSmallDevices(t *testing.T) {
	plan, err := Plan(32*1024, 64, 4096, DefaultGuardBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 8 {
		t.Fatalf("samples=%d want=8", len(plan))
	}
	if plan[0].Offset != 0 || plan[len(plan)-1].Offset != 28*1024 {
		t.Fatalf("unexpected endpoints: first=%d last=%d", plan[0].Offset, plan[len(plan)-1].Offset)
	}
}

func TestPlanRejectsUnsafeConfiguration(t *testing.T) {
	if _, err := Plan(1024, 8, 4096, 0); err == nil {
		t.Fatal("expected too-small-device error")
	}
	if _, err := Plan(64*1024, 1, 4096, 0); err == nil {
		t.Fatal("expected sample-count error")
	}
	if _, err := Plan(64*1024, MaxSamples+1, 4096, 0); err == nil {
		t.Fatal("expected maximum sample-count error")
	}
}

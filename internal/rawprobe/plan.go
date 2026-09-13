package rawprobe

import (
	"errors"
	"fmt"
	"math"
)

const (
	DefaultBlockBytes = 4096
	DefaultSamples    = 64
	DefaultGuardBytes = 4 * 1024 * 1024
	MaxSamples        = 4096
)

type Sample struct {
	Index  int    `json:"index"`
	Offset uint64 `json:"offsetBytes"`
}

// Plan distributes aligned samples across the advertised address space. A
// small guard is kept at each edge when possible so an interrupted run is less
// likely to damage partition metadata. This is risk reduction, not a safety
// guarantee: raw probing still writes directly to the block device.
func Plan(capacity uint64, samples int, blockBytes uint64, guardBytes uint64) ([]Sample, error) {
	if blockBytes == 0 {
		return nil, errors.New("block size must be greater than zero")
	}
	if samples < 2 || samples > MaxSamples {
		return nil, fmt.Errorf("sample count must be between 2 and %d", MaxSamples)
	}
	if capacity > math.MaxInt64 {
		return nil, errors.New("capacity exceeds supported signed offsets")
	}
	if capacity < blockBytes*2 {
		return nil, errors.New("device is too small for raw probing")
	}

	alignUp := func(v uint64) uint64 {
		if rem := v % blockBytes; rem != 0 {
			return v + (blockBytes - rem)
		}
		return v
	}
	alignDown := func(v uint64) uint64 { return v - (v % blockBytes) }

	maxOffset := alignDown(capacity - blockBytes)
	start := alignUp(guardBytes)
	end := maxOffset
	if guardBytes < maxOffset {
		candidate := alignDown(maxOffset - guardBytes)
		if candidate > start {
			end = candidate
		}
	}
	if start >= end {
		start = 0
		end = maxOffset
	}

	slots := (end-start)/blockBytes + 1
	if slots < 2 {
		return nil, errors.New("device does not contain two aligned probe locations")
	}
	if uint64(samples) > slots {
		samples = int(slots)
	}

	denom := uint64(samples - 1)
	spanSlots := slots - 1
	q := spanSlots / denom
	r := spanSlots % denom
	out := make([]Sample, samples)
	for i := 0; i < samples; i++ {
		u := uint64(i)
		slot := u*q + (u*r)/denom
		out[i] = Sample{Index: i, Offset: start + slot*blockBytes}
		if i > 0 && out[i].Offset <= out[i-1].Offset {
			return nil, errors.New("probe planner generated duplicate offsets")
		}
	}
	return out, nil
}

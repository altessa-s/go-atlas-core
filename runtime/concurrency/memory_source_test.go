// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
)

func TestMemorySource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		mb    uint64
		known bool
		want  int
	}{
		{"unknown", 0, false, concurrency.DefaultLimitFunc()},
		{"exhausted", 0, true, 1},
		{"low", 200, true, runtime.NumCPU()},
		{"medium", 600, true, runtime.NumCPU() * 2},
		{"high", 2000, true, runtime.NumCPU() * 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := func() (uint64, bool) { return tc.mb << 20, tc.known }
			fn := concurrency.MemoryAwareConcurrency(100, 500, 1000, concurrency.WithAvailableMemory(source))
			require.Equal(t, tc.want, fn())
		})
	}
	unknown := func() (uint64, bool) { return 0, false }
	adaptive := concurrency.AdaptiveConcurrency(100, 500, nil, 0, concurrency.WithAvailableMemory(unknown))
	require.Equal(t, concurrency.DefaultLimitFunc(), adaptive())
}

func TestAdaptiveConcurrency_Factors(t *testing.T) {
	t.Parallel()

	base := runtime.NumCPU() * concurrency.DefaultConcurrencyMultiplier
	clamp := func(v int) int { return min(max(v, concurrency.MinConcurrencyLimit), concurrency.MaxConcurrencyLimit) }

	tests := []struct {
		name          string
		memoryLowMB   uint64
		availableMB   uint64
		known         bool
		load          float64
		loadThreshold float64
		want          int
	}{
		{name: "no pressure", memoryLowMB: 100, availableMB: 4096, known: true, want: clamp(base)},
		{name: "severe memory pressure", memoryLowMB: 100, availableMB: 50, known: true, want: clamp(base / 4)},
		{name: "medium memory pressure", memoryLowMB: 100, availableMB: 300, known: true, want: clamp(base / 2)},
		{name: "memory factor disabled", memoryLowMB: 0, availableMB: 0, known: true, want: clamp(base)},
		{name: "unknown memory skips penalty", memoryLowMB: 100, availableMB: 0, known: false, want: clamp(base)},
		{name: "severe load", memoryLowMB: 0, load: 9, loadThreshold: 8, want: clamp(base / 4)},
		{name: "medium load", memoryLowMB: 0, load: 5, loadThreshold: 8, want: clamp(base / 2)},
		{name: "low load", memoryLowMB: 0, load: 1, loadThreshold: 8, want: clamp(base)},
		{name: "load factor disabled by threshold", memoryLowMB: 0, load: 100, loadThreshold: 0, want: clamp(base)},
		{
			name: "memory and load combine", memoryLowMB: 100, availableMB: 50, known: true,
			load: 9, loadThreshold: 8, want: clamp(base / 16),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := func() (uint64, bool) { return tc.availableMB << 20, tc.known }
			load := func() float64 { return tc.load }
			fn := concurrency.AdaptiveConcurrency(tc.memoryLowMB, 500, load, tc.loadThreshold, concurrency.WithAvailableMemory(source))
			require.Equal(t, tc.want, fn())
		})
	}
}

func TestAdaptiveConcurrency_NilLoadIgnored(t *testing.T) {
	t.Parallel()

	fn := concurrency.AdaptiveConcurrency(0, 0, nil, 1)
	require.Equal(t, min(runtime.NumCPU()*concurrency.DefaultConcurrencyMultiplier, concurrency.MaxConcurrencyLimit), fn())
}

func TestWithAvailableMemory_NilKeepsDefaultSource(t *testing.T) {
	t.Parallel()

	fn := concurrency.MemoryAwareConcurrency(100, 500, 1000, concurrency.WithAvailableMemory(nil))
	require.GreaterOrEqual(t, fn(), concurrency.MinConcurrencyLimit, "a nil source must not replace the default one")
}

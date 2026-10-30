// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package maps_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

// fillMap stores n distinct entries in m.
func fillMap(tb testing.TB, m map[int]int, n int) {
	tb.Helper()
	for i := range n {
		m[i] = i
	}
}

func TestPoolGetReturnsEmptyMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		defaultCap int
	}{
		{name: "positive capacity", defaultCap: 8},
		{name: "zero capacity falls back to default", defaultCap: 0},
		{name: "negative capacity falls back to default", defaultCap: -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool := coremaps.NewPool[int, int](tc.defaultCap)
			m := pool.Get()
			require.NotNil(t, m)
			require.NotNil(t, *m)
			require.Empty(t, *m)
		})
	}
}

func TestPoolGetAfterPutIsEmpty(t *testing.T) {
	t.Parallel()
	pool := coremaps.NewPool[int, int](4)
	m := pool.Get()
	fillMap(t, *m, 10)
	pool.Put(m)

	got := pool.Get()
	require.Empty(t, *got)
}

func TestPoolPut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries int
		wantLen int
	}{
		{name: "below threshold is cleared", entries: 10, wantLen: 0},
		{name: "at threshold is cleared", entries: 1024, wantLen: 0},
		// Oversized maps are discarded before clearing, so their contents survive.
		{name: "above threshold is discarded", entries: 1025, wantLen: 1025},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool := coremaps.NewPool[int, int](4)
			m := pool.Get()
			fillMap(t, *m, tc.entries)
			// Inspect the released map directly: sync.Pool may drop it, so a
			// later Get cannot reliably show whether Put cleared or discarded it.
			released := *m
			pool.Put(m)
			require.Len(t, released, tc.wantLen)
		})
	}
}

func TestPoolPutNil(t *testing.T) {
	t.Parallel()
	pool := coremaps.NewPool[int, int](4)
	require.NotPanics(t, func() { pool.Put(nil) })
}

func TestPoolGetWithCapacity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected int
	}{
		{name: "within default capacity", expected: 8},
		{name: "above default capacity", expected: 512},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool := coremaps.NewPool[int, int](8)
			seed := pool.Get()
			fillMap(t, *seed, 4)
			pool.Put(seed)

			m := pool.GetWithCapacity(tc.expected)
			require.NotNil(t, m)
			require.NotNil(t, *m)
			require.Empty(t, *m)
			fillMap(t, *m, tc.expected)
			require.Len(t, *m, tc.expected)
			pool.Put(m)
		})
	}
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package maps_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"

	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

func TestMapIter_EarlyBreakStopsTransforming(t *testing.T) {
	t.Parallel()

	m := map[int]int{1: 1, 2: 2, 3: 3, 4: 4}
	var calls int
	seq := coremaps.Map(m, func(k, v int) (int, int) {
		calls++
		return k, v * 10
	})
	var yielded int
	for k, v := range seq {
		require.Equal(t, k*10, v)
		yielded++
		if yielded == 2 {
			break
		}
	}
	require.Equal(t, 2, yielded)
	require.Equal(t, 2, calls)
}

func TestPool_ZeroValueGet(t *testing.T) {
	t.Parallel()

	var pool coremaps.Pool[string, int]
	m := pool.Get()
	require.NotNil(t, m)
	require.NotNil(t, *m)
	require.Empty(t, *m)

	(*m)["k"] = 1
	pool.Put(m)

	again := pool.Get()
	require.NotNil(t, again)
	require.Empty(t, *again)
}

func TestWeakMap_SetNilDeletes(t *testing.T) {
	t.Parallel()

	wm := coremaps.NewWeakMap[string, int]()
	v := 1
	wm.Set("k", &v)
	require.Equal(t, 1, wm.Len())

	wm.Set("k", nil)
	_, ok := wm.Get("k")
	require.False(t, ok)
	require.Zero(t, wm.Len())
	runtime.KeepAlive(&v)
}

func TestWeakMap_RangeStopsEarly(t *testing.T) {
	t.Parallel()

	wm := coremaps.NewWeakMap[int, int]()
	vals := []int{0, 1, 2, 3}
	for i := range vals {
		wm.Set(i, &vals[i])
	}

	var visited int
	wm.Range(func(int, *int) bool {
		visited++
		return false
	})
	require.Equal(t, 1, visited)
	runtime.KeepAlive(vals)
}

func TestWeakMap_RangeAndCleanupDropCollectedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		sweep func(wm *coremaps.WeakMap[int, testhelpers.User]) []int
	}{
		{
			name: "Range",
			sweep: func(wm *coremaps.WeakMap[int, testhelpers.User]) []int {
				var keys []int
				wm.Range(func(k int, v *testhelpers.User) bool {
					require.NotNil(t, v)
					keys = append(keys, k)
					return true
				})
				return keys
			},
		},
		{
			name: "Cleanup",
			sweep: func(wm *coremaps.WeakMap[int, testhelpers.User]) []int {
				wm.Cleanup()
				return nil
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wm := coremaps.NewWeakMap[int, testhelpers.User]()
			for i := range 20 {
				wm.Set(i, &testhelpers.User{ID: i})
			}
			kept := &testhelpers.User{ID: 100}
			wm.Set(100, kept)

			for range 3 {
				runtime.GC()
			}

			keys := tc.sweep(wm)
			if keys != nil {
				require.Equal(t, []int{100}, keys)
			}
			// Whether the sweep or the asynchronous cleanup callback removed a
			// collected entry, none may remain once the sweep returns.
			require.Equal(t, 1, wm.Len())
			got, ok := wm.Get(100)
			require.True(t, ok)
			require.Same(t, kept, got)
			runtime.KeepAlive(kept)
		})
	}
}

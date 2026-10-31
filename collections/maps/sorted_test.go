// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package maps_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/collections/maps"
)

func TestToSortedKeyValueSlice(t *testing.T) {
	t.Parallel()
	require.Nil(t, maps.ToSortedKeyValueSlice(map[string]int(nil)))
	require.Nil(t, maps.ToSortedKeyValueSlice(map[string]int{}))
	require.Equal(t, []any{"a", 1, "b", 2, "c", 3}, maps.ToSortedKeyValueSlice(map[string]int{"c": 3, "a": 1, "b": 2}))
	got := maps.ToSortedKeyValueSlice(map[float64]string{math.NaN(): "nan", 1: "one"})
	require.True(t, math.IsNaN(got[0].(float64)))
	require.Equal(t, []any{"nan", float64(1), "one"}, got[1:])
}

func BenchmarkToSortedKeyValueSlice(b *testing.B) {
	m := map[string]int{"c": 3, "a": 1, "b": 2}
	b.ReportAllocs()
	for b.Loop() {
		_ = maps.ToSortedKeyValueSlice(m)
	}
}

func FuzzToSortedKeyValueSlice(f *testing.F) {
	f.Add([]byte{3, 1, 2, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		m := make(map[byte]int)
		for _, k := range data {
			m[k]++
		}
		got := maps.ToSortedKeyValueSlice(m)
		require.Len(t, got, 2*len(m))
		for i := 0; i < len(got); i += 2 {
			k := got[i].(byte)
			require.Equal(t, m[k], got[i+1])
			if i > 0 {
				require.Less(t, got[i-2].(byte), k)
			}
		}
	})
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package maps_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

func TestFromFlatMapCopiesNestedMaps(t *testing.T) {
	t.Parallel()

	nested := map[string]any{"leaf": map[string]any{"value": 1}}
	got := coremaps.FromFlatMap(map[string]any{"root": nested})
	root := got["root"].(map[string]any)
	root["extra"] = true
	root["leaf"].(map[string]any)["value"] = 2
	require.Equal(t, map[string]any{"leaf": map[string]any{"value": 1}}, nested)
}

func TestFromFlatMapExpandsMapValues(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		base map[string]any
		path string
		want map[string]any
	}{
		{name: "nil map", path: "root.child", want: map[string]any{"child": 2}},
		{name: "nested nil map", base: map[string]any{"nested": map[string]any(nil)}, path: "root.nested.child",
			want: map[string]any{"nested": map[string]any{"child": 2}}},
		{name: "merge and override", base: map[string]any{"child": 1, "sibling": 3}, path: "root.child",
			want: map[string]any{"child": 2, "sibling": 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := map[string]any{"root": tc.base, tc.path: 2}
			// Exercise different map iteration orders; parent values must always
			// be installed before their explicit dotted descendants.
			for range 32 {
				var got map[string]any
				require.NotPanics(t, func() { got = coremaps.FromFlatMap(in) })
				require.Equal(t, map[string]any{"root": tc.want}, got)
			}
		})
	}
}

func TestFromFlatMapConflictsAreDeterministic(t *testing.T) {
	t.Parallel()

	in := map[string]any{"root": 1, "root.a": 2, "root.b.c": 3, "sibling": 4}
	for range 32 {
		var conflicts []string
		got := coremaps.FromFlatMapWithHandler(in, func(key string, existing any) {
			require.Equal(t, 1, existing)
			conflicts = append(conflicts, key)
		})
		require.Equal(t, map[string]any{"root": 1, "sibling": 4}, got)
		require.Equal(t, []string{"root.a", "root.b.c"}, conflicts)
	}
}

func TestFlatMapPreservesEmptySegments(t *testing.T) {
	t.Parallel()

	nested := map[string]any{
		"":  map[string]any{"": 1, "child": 2},
		"x": map[string]any{"": 3},
		"y": map[string]any{"z": 4},
	}
	flat := map[string]any{".": 1, ".child": 2, "x.": 3, "y.z": 4}
	require.Equal(t, flat, coremaps.ToFlatMap(nested, nil))
	require.Equal(t, nested, coremaps.FromFlatMap(flat))
	require.Equal(t, map[string]any{".child": 1}, coremaps.ToFlatMap(
		map[string]any{"root": map[string]any{"child": 1}},
		func(key string) string {
			if key == "root" {
				return ""
			}
			return key
		}))
}

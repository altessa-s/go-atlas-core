// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package capabilities_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/capabilities"
)

var allKinds = []capabilities.SetKind{
	capabilities.Effective,
	capabilities.Permitted,
	capabilities.Inheritable,
	capabilities.Bounding,
	capabilities.Ambient,
}

func TestSets_WithThenHasPerKind(t *testing.T) {
	t.Parallel()

	const c = capabilities.Cap(10)
	for _, kind := range allKinds {
		s := capabilities.Sets{}.With(c, kind)
		for _, other := range allKinds {
			require.Equal(t, other == kind, s.Has(c, other), "set %d, query %d", kind, other)
		}
		require.False(t, s.Has(c+1, kind), "only the requested bit is set")
		require.Equal(t, kind == capabilities.Effective, s.Has(c), "zero-kind Has queries Effective")
	}
}

func TestSets_WithSetsExpectedBits(t *testing.T) {
	t.Parallel()

	s := capabilities.Sets{}.With(3, allKinds...)
	want := uint64(1) << 3
	require.Equal(t, capabilities.Sets{Effective: want, Permitted: want, Inheritable: want, Bounding: want, Ambient: want}, s)

	s = s.With(63, capabilities.Permitted)
	require.Equal(t, want|uint64(1)<<63, s.Permitted)
	require.True(t, s.Has(63, capabilities.Effective, capabilities.Permitted), "any matching kind suffices")
	require.False(t, s.Has(63, capabilities.Effective, capabilities.Ambient))
}

func TestSets_WithDoesNotMutateReceiver(t *testing.T) {
	t.Parallel()

	orig := capabilities.Sets{Effective: 1}
	_ = orig.With(5, capabilities.Effective)
	require.Equal(t, capabilities.Sets{Effective: 1}, orig)
}

func TestSets_OutOfRangeAndUnknownKinds(t *testing.T) {
	t.Parallel()

	full := capabilities.Sets{Effective: ^uint64(0), Permitted: ^uint64(0)}
	require.False(t, full.Has(-1))
	require.False(t, full.Has(64))
	require.False(t, full.Has(1, capabilities.SetKind(0), capabilities.SetKind(99)), "unknown kinds are ignored")

	require.Equal(t, capabilities.Sets{}, capabilities.Sets{}.With(-1, allKinds...))
	require.Equal(t, capabilities.Sets{}, capabilities.Sets{}.With(64, allKinds...))
	require.Equal(t, capabilities.Sets{}, capabilities.Sets{}.With(2), "no kinds is a no-op")
	require.Equal(t, capabilities.Sets{}, capabilities.Sets{}.With(2, capabilities.SetKind(42)))
}

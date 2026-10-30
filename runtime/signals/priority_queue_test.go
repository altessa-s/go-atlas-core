// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package signals

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPriorityQueueCustomPriorityOrdering proves that a handler registered
// with a custom priority that matches no predefined bucket is ordered
// relative to the predefined levels instead of being misfiled into the
// highest-priority bucket: a custom priority of 10 must run after
// PriorityLow (25) and before PriorityLowest (1), never first.
func TestPriorityQueueCustomPriorityOrdering(t *testing.T) {
	t.Parallel()

	const customLow = Priority(10)

	pq := newPriorityQueue()
	entries := makeHandlersWithPriorities([]Priority{
		PriorityLow, PriorityHighest, customLow, PriorityNormal, customLow, PriorityLowest,
	})
	for _, e := range entries {
		pq.addHandler(e)
	}

	got := pq.getHandlersInOrder()
	require.Len(t, got, len(entries))
	assertSortedDescending(t, got)
	assertStable(t, got)

	priorities := make([]Priority, len(got))
	for i, h := range got {
		priorities[i] = h.priority
	}
	require.Equal(t,
		[]Priority{PriorityHighest, PriorityNormal, PriorityLow, customLow, customLow, PriorityLowest},
		priorities)
}

// TestPriorityQueueMixedPrioritiesMatchSort proves that the bucket queue
// produces exactly the order of the stable sort path for mixed predefined and
// custom priorities, including values above PriorityHighest, below
// PriorityLowest, zero and negative, registered in interleaved order.
func TestPriorityQueueMixedPrioritiesMatchSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		priorities []Priority
	}{
		{name: "empty", priorities: nil},
		{name: "custom only", priorities: []Priority{60, 10, 60, 90, 10}},
		{name: "above highest and below lowest", priorities: []Priority{
			PriorityNormal, 200, PriorityHighest, 0, PriorityLowest, 200, -5, PriorityHighest, 0,
		}},
		{name: "interleaved", priorities: []Priority{
			10, PriorityLow, 99, PriorityHighest, 51, PriorityNormal, 49, 99, PriorityHigh, 26, PriorityLow, 24, 10,
			PriorityLowest, 2, 51, -1, 101,
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			entries := makeHandlersWithPriorities(tc.priorities)
			pq := newPriorityQueue()
			for _, e := range entries {
				pq.addHandler(e)
			}
			got := pq.getHandlersInOrder()

			want := slices.Clone(entries)
			(&Signal{}).sortHandlersByPriority(want)

			require.Len(t, got, len(want))
			for i := range want {
				require.Equal(t, want[i].priority, got[i].priority, "priority at i=%d", i)
				require.Equal(t, want[i].timeout, got[i].timeout, "identity at i=%d", i)
			}
			assertSortedDescending(t, got)
			assertStable(t, got)
		})
	}
}

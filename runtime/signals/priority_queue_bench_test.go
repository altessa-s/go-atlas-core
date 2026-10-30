// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package signals

import (
	"strconv"
	"testing"
)

// BenchmarkPriorityQueue measures the full large-set dispatch path (build the
// queue, then read it back in order) for predefined-only, mixed and
// custom-only priority distributions.
func BenchmarkPriorityQueue(b *testing.B) {
	predefined := []Priority{PriorityHighest, PriorityHigh, PriorityNormal, PriorityLow, PriorityLowest}
	custom := []Priority{90, 60, 40, 10, 5}
	mixed := append(append([]Priority{}, predefined...), custom...)

	cases := []struct {
		name string
		pool []Priority
	}{
		{name: "predefined", pool: predefined},
		{name: "mixed", pool: mixed},
		{name: "custom", pool: custom},
	}

	for _, tc := range cases {
		for _, n := range []int{100, 500} {
			b.Run(tc.name+"/n="+strconv.Itoa(n), func(b *testing.B) {
				priorities := make([]Priority, n)
				for i := range priorities {
					priorities[i] = tc.pool[i%len(tc.pool)]
				}
				entries := makeHandlersWithPriorities(priorities)

				b.ReportAllocs()
				for b.Loop() {
					pq := newPriorityQueue()
					for _, e := range entries {
						pq.addHandler(e)
					}
					_ = pq.getHandlersInOrder()
				}
			})
		}
	}
}

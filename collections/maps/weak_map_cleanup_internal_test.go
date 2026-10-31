// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package maps

import (
	"runtime"
	"testing"
)

type observedCleanup struct {
	cleanup interface{ Stop() }
	stops   int
}

func (c *observedCleanup) Stop() {
	c.stops++
	c.cleanup.Stop()
}

// Observe Stop directly: process-wide memory statistics and GC scheduling cannot
// reliably tell us whether a replacement stopped its predecessor's cleanup.
func TestWeakMap_ReplaceStopsPreviousCleanup(t *testing.T) {
	t.Parallel()

	m := NewWeakMap[int, [32]byte]()
	a, b := new([32]byte), new([32]byte)
	defer runtime.KeepAlive(a)
	defer runtime.KeepAlive(b)
	defer func() {
		for key := range 16 {
			m.Delete(key)
		}
	}()

	var cleanups []*observedCleanup
	for i := range 128 {
		key := i % 16
		val := a
		if i/16%2 != 0 {
			val = b
		}
		m.Set(key, val)

		// Both values remain live throughout the test, so no cleanup callback
		// can access the map while we wrap the newly registered handle.
		entry := m.data[key]
		current := &observedCleanup{cleanup: entry.cleanup}
		entry.cleanup = current
		m.data[key] = entry
		cleanups = append(cleanups, current)

		// Exactly the latest cleanup for each key must remain unstopped.
		for j, cleanup := range cleanups {
			want := 0
			if j <= i-16 {
				want = 1
			}
			if cleanup.stops != want {
				t.Fatalf("after Set %d: cleanup %d stopped %d times, want %d", i, j, cleanup.stops, want)
			}
		}
	}
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package concurrency

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryHeadroom(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		limit               int64
		sys, released, want uint64
		known               bool
	}{
		{"budget", 100, 80, 20, 40, true},
		{"exhausted", 50, 80, 20, 0, true},
		{"zero", 0, 80, 20, 0, true},
		{"unknown", -1, 80, 20, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, known := memoryHeadroom(&runtime.MemStats{Sys: tc.sys, HeapReleased: tc.released}, tc.limit)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.known, known)
		})
	}
}

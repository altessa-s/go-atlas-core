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

// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package slices_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"

	coreslices "github.com/altessa-s/go-atlas-core/collections/slices"
)

// parallelInput returns a slice large enough to take the parallel path of
// MapParallel and FilterParallel (at least batchSize * NumCPU * 2 elements).
func parallelInput() []int {
	in := make([]int, 64*runtime.NumCPU()*4)
	for i := range in {
		in[i] = i
	}
	return in
}

func TestMapParallel_NoLeakedWorkers(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)

	in := parallelInput()
	out := coreslices.MapParallel(in, func(v int) int { return v * 2 })
	require.Len(t, out, len(in))
	require.Equal(t, 2*(len(in)-1), out[len(out)-1])
}

func TestFilterParallel_NoLeakedWorkers(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)

	in := parallelInput()
	out := coreslices.FilterParallel(in, func(v int) bool { return v%2 == 0 })
	require.Len(t, out, len(in)/2)
	require.Equal(t, len(in)-2, out[len(out)-1])
}

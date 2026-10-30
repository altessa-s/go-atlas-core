// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"context"
	"math/rand/v2"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"
	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
)

func TestProcessCollect_Ordering(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)

	// Create a list of integers
	count := 100
	input := make([]int, count)
	for i := range count {
		input[i] = i
	}

	// The bubble's fake clock makes the random delays free while still
	// completing items out of order: a worker wakes when its own delay
	// expires, not in claim order.
	synctest.Test(t, func(t *testing.T) {
		results, err := concurrency.ProcessCollect(t.Context(), input, func(ctx context.Context, item int) (int, error) {
			// Random delay between 0 and 10ms
			time.Sleep(time.Duration(rand.IntN(10)) * time.Millisecond)
			return item, nil
		}, concurrency.WithConcurrency[int](10))

		require.NoError(t, err)
		require.Equal(t, input, results, "ProcessCollect did not preserve order")
	})
}

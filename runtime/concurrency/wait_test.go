// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
)

func TestWaitContext(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	require.NoError(t, concurrency.WaitContext(t.Context(), &wg), "an idle group returns at once")

	release := make(chan struct{})
	wg.Go(func() { <-release })
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, concurrency.WaitContext(ctx, &wg), context.DeadlineExceeded, "the context bounds the wait")

	close(release)
	require.NoError(t, concurrency.WaitContext(t.Context(), &wg))
}

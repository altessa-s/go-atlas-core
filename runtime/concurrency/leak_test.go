// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"
	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
)

// The leak tests exercise the paths that stop work early; every worker and
// helper goroutine they start must have exited when the call returns.

func TestProcess_StopOnError_NoLeakedWorkers(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)

	errBoom := errors.New("boom")
	items := make([]int, 100)
	err := concurrency.Process(t.Context(), items, func(ctx context.Context, _ int) error {
		return errBoom
	}, concurrency.WithConcurrency[int](8), concurrency.WithStopOnError[int]())
	require.ErrorIs(t, err, errBoom)
}

func TestProcessCollect_Canceled_NoLeakedWorkers(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)

	ctx, cancel := context.WithCancel(t.Context())
	items := make([]int, 100)
	_, err := concurrency.ProcessCollect(ctx, items, func(ctx context.Context, item int) (int, error) {
		cancel()
		<-ctx.Done()
		return 0, ctx.Err()
	}, concurrency.WithConcurrency[int](8))
	require.ErrorIs(t, err, context.Canceled)
}

func TestWaitContext_Timeout_WaiterExitsWhenGroupFinishes(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)

	var wg sync.WaitGroup
	release := make(chan struct{})
	wg.Go(func() { <-release })

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, concurrency.WaitContext(ctx, &wg), context.DeadlineExceeded)

	// WaitContext's waiter goroutine is still blocked on wg here; it is not
	// a leak because the group can still finish, and it exits once it does.
	close(release)
	require.NoError(t, concurrency.WaitContext(t.Context(), &wg))
}

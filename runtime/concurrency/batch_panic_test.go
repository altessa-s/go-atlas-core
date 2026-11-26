// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
	"github.com/altessa-s/go-atlas-core/runtime/panics"
)

func TestBatchWorkerPanic(t *testing.T) {
	t.Parallel()

	process := func(ctx context.Context, items []int, fn concurrency.TransformFunc[int, int], opts ...concurrency.Option[int]) ([]int, error) {
		return nil, concurrency.Process(ctx, items, func(ctx context.Context, item int) error {
			_, err := fn(ctx, item)
			return err
		}, opts...)
	}

	for _, batch := range []struct {
		name    string
		run     func(context.Context, []int, concurrency.TransformFunc[int, int], ...concurrency.Option[int]) ([]int, error)
		collect bool
	}{
		{name: "process", run: process},
		{name: "collect", run: concurrency.ProcessCollect[int, int], collect: true},
	} {
		t.Run(batch.name, func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				name    string
				workers int
				stop    bool
				want    []int
			}{
				{name: "sequential/continue", workers: 1, want: []int{0, 2, 3}},
				{name: "sequential/stop", workers: 1, stop: true, want: []int{0}},
				{name: "parallel/continue", workers: 2, want: []int{0, 2, 3}},
				{name: "parallel/stop", workers: 2, stop: true, want: []int{0, 2}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()

					payload := errors.New("private worker panic value")
					started := make(chan struct{})

					var (
						successes, failures []int
						itemErr             error
					)

					opts := []concurrency.Option[int]{
						concurrency.WithConcurrency[int](tc.workers),
						concurrency.WithOnSuccess[int](func(item int) { successes = append(successes, item) }),
						concurrency.WithOnError[int](func(item int, err error) {
							failures = append(failures, item)
							itemErr = err
						}),
					}
					if tc.stop {
						opts = append(opts, concurrency.WithStopOnError[int]())
					}

					results, err := batch.run(ctx, []int{0, 1, 2, 3}, func(ctx context.Context, item int) (int, error) {
						if tc.stop && tc.workers > 1 {
							switch item {
							case 1:
								select {
								case <-started:
								case <-ctx.Done():
									return 0, ctx.Err()
								}
							case 2:
								close(started)
								<-ctx.Done()
							}
						}

						if item == 1 {
							panic(payload)
						}

						return item, nil
					}, opts...)

					require.Equal(t, []int{1}, failures)
					require.ElementsMatch(t, tc.want, successes)
					require.EqualError(t, itemErr, "worker panicked")
					require.ErrorIs(t, itemErr, concurrency.ErrWorkerPanic)
					require.ErrorIs(t, err, concurrency.ErrWorkerPanic)
					require.ErrorIs(t, err, itemErr)
					require.NotErrorIs(t, err, payload)

					var traced *panics.RecoveredError
					require.ErrorAs(t, err, &traced)
					require.Contains(t, traced.StackTrace(), "batch_panic_test.go")
					require.NotContains(t, traced.StackTrace(), payload.Error())

					if batch.collect {
						require.Equal(t, tc.want, results)
					}
				})
			}
		})
	}
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
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
					require.ErrorIs(t, itemErr, concurrency.ErrWorkerPanic)
					require.ErrorIs(t, err, concurrency.ErrWorkerPanic)
					require.ErrorIs(t, err, itemErr)
					require.NotErrorIs(t, err, payload)

					if batch.collect {
						require.Equal(t, tc.want, results)
					}
				})
			}
		})
	}
}

func TestBatchWorkerPanicReportsClass(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	panics.SetReallyPanic(true)
	t.Cleanup(func() { panics.SetReallyPanic(false) })

	type reportKey struct{ _ byte }

	key := new(reportKey)
	reports := make(chan any, 8)
	ctx := context.WithValue(t.Context(), key, reports)

	panics.AddGlobalPanicHandler(func(c context.Context, value any) {
		if ch, ok := c.Value(key).(chan any); ok {
			ch <- value
		}
	})

	for _, workers := range []int{1, 2} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			payload := errors.New("private worker panic value")
			results, err := concurrency.ProcessCollect(ctx, []int{0, 1, 2, 3}, func(_ context.Context, item int) (int, error) {
				switch item {
				case 0:
					panic(payload)
				case 2:
					panic(nil)
				}

				return item, nil
			}, concurrency.WithConcurrency[int](workers))

			require.ErrorIs(t, err, concurrency.ErrWorkerPanic)
			require.Equal(t, []int{1, 3}, results)
			require.Len(t, reports, 2)
			require.ElementsMatch(t, []any{fmt.Sprintf("%T", payload), "<nil>"}, []any{<-reports, <-reports})
		})
	}

	t.Run("Goexit/sequential", func(t *testing.T) {
		var returned bool

		done := make(chan struct{})
		go func() {
			defer close(done)

			_ = concurrency.Process(ctx, []int{0}, func(context.Context, int) error {
				runtime.Goexit()
				return nil
			})
			returned = true
		}()

		<-done
		require.False(t, returned)
		require.Empty(t, reports)
	})

	t.Run("Goexit/parallel", func(t *testing.T) {
		results, err := concurrency.ProcessCollect(ctx, []int{1, 2, 3}, func(_ context.Context, item int) (int, error) {
			if item == 1 {
				runtime.Goexit()
			}

			return item, nil
		}, concurrency.WithConcurrency[int](2))
		require.NoError(t, err)
		require.Equal(t, []int{0, 2, 3}, results)
		require.Empty(t, reports)
	})
}

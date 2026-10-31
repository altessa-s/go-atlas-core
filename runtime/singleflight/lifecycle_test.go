// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package singleflight_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"
	"github.com/altessa-s/go-atlas-core/runtime/singleflight"
)

func TestSharedAndForget(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	synctest.Test(t, func(t *testing.T) {
		g := singleflight.New[string, int]()
		release := make(chan struct{})
		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() {
				v, shared, err := g.DoShared(t.Context(), "k", func(context.Context) (int, error) { <-release; return 1, nil })
				if err != nil || !shared || v != 1 {
					t.Errorf("shared result = %d, %v, %v", v, shared, err)
				}
			})
			synctest.Wait()
		}
		g.Forget("k")
		nextRelease := make(chan struct{})
		wg.Go(func() {
			_, _ = g.Do(t.Context(), "k", func(context.Context) (int, error) { <-nextRelease; return 2, nil })
		})
		synctest.Wait()
		close(release)
		synctest.Wait()
		// Completing the forgotten call must not remove its replacement.
		wg.Go(func() {
			v, shared, err := g.DoShared(t.Context(), "k", func(context.Context) (int, error) { panic("replacement lost") })
			if err != nil || !shared || v != 2 {
				t.Errorf("replacement = %d, %v, %v", v, shared, err)
			}
		})
		synctest.Wait()
		g.Close()
		g.Close()
		_, err := g.Do(t.Context(), "k", func(context.Context) (int, error) { panic("closed") })
		require.ErrorIs(t, err, singleflight.ErrClosed)
		close(nextRelease)
		wg.Wait()
		g.Wait()
	})
}

func TestLifecycleWaiters(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	synctest.Test(t, func(t *testing.T) {
		parent, stop := context.WithCancel(t.Context())
		defer stop()
		g := singleflight.New[string, int](singleflight.WithLifecycle(parent))
		first, cancelFirst := context.WithCancel(t.Context())
		second, cancelSecond := context.WithCancel(t.Context())
		var work context.Context
		var wg sync.WaitGroup
		fn := func(ctx context.Context) (int, error) { work = ctx; <-ctx.Done(); return 0, ctx.Err() }
		wg.Go(func() { _, _ = g.Do(first, "k", fn) })
		synctest.Wait()
		wg.Go(func() { _, _ = g.Do(second, "k", fn) })
		synctest.Wait()
		cancelFirst()
		synctest.Wait()
		require.NoError(t, work.Err())
		cancelSecond()
		synctest.Wait()
		require.ErrorIs(t, work.Err(), context.Canceled)
		wg.Wait()
		v, shared, err := g.DoShared(t.Context(), "k", func(context.Context) (int, error) { return 9, nil })
		require.NoError(t, err)
		require.False(t, shared)
		require.Equal(t, 9, v)
		stop()
		_, err = g.Do(t.Context(), "parent", fn)
		require.ErrorIs(t, err, context.Canceled)
		g.Close()
		g.Wait()
	})
}

func TestPanicAsError(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	want := errors.New("redacted panic")
	g := singleflight.New[string, int](singleflight.WithPanicAsError(want))
	v, shared, err := g.DoShared(t.Context(), "k", func(context.Context) (int, error) { panic("secret") })
	require.Zero(t, v)
	require.False(t, shared)
	require.Same(t, want, err)
	g.Close()
	g.Wait()
}

func TestForgottenWorkHoldsCapacity(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	synctest.Test(t, func(t *testing.T) {
		g := singleflight.New[string, int](singleflight.WithMaxConcurrent(1))
		release := make(chan struct{})
		go func() { _, _ = g.Do(t.Context(), "k", func(context.Context) (int, error) { <-release; return 0, nil }) }()
		synctest.Wait()
		g.Forget("k")
		_, err := g.Do(t.Context(), "k", func(context.Context) (int, error) { panic("over capacity") })
		require.ErrorIs(t, err, singleflight.ErrOverloaded)
		g.Close()
		waited := make(chan struct{})
		go func() { g.Wait(); close(waited) }()
		synctest.Wait()
		select {
		case <-waited:
			t.Fatal("Wait ignored forgotten work")
		default:
		}
		close(release)
		<-waited
	})
}

func TestLifecycleParentCancelsActiveWork(t *testing.T) {
	t.Parallel()
	testhelpers.VerifyNoLeaks(t)
	synctest.Test(t, func(t *testing.T) {
		parent, cancel := context.WithCancel(t.Context())
		defer cancel()
		g := singleflight.New[string, int](singleflight.WithLifecycle(parent))
		result := make(chan error, 1)
		go func() {
			_, err := g.Do(t.Context(), "k", func(work context.Context) (int, error) {
				<-work.Done()
				return 0, work.Err()
			})
			result <- err
		}()
		synctest.Wait()
		g.Close()
		cancel()
		g.Wait()
		require.ErrorIs(t, <-result, context.Canceled)
	})
}

func TestPanicPolicyWithJoinedWaiters(t *testing.T) {
	t.Parallel()
	want := errors.New("redacted panic")
	for _, tc := range []struct {
		name      string
		option    error
		goexit    bool
		wantErr   error
		wantPanic any
	}{
		{name: "redacted", option: want, wantErr: want},
		{name: "goexit", option: want, goexit: true, wantErr: singleflight.ErrAborted},
		{name: "nil option", wantPanic: "secret"},
		{name: "nil option goexit", goexit: true, wantErr: singleflight.ErrAborted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testhelpers.VerifyNoLeaks(t)
			synctest.Test(t, func(t *testing.T) {
				g := singleflight.New[string, int](singleflight.WithPanicAsError(tc.option))
				release := make(chan struct{})
				type result struct {
					value      int
					shared     bool
					err        error
					panicValue any
				}
				results := make(chan result, 2)
				var executions atomic.Int64
				fn := func(context.Context) (int, error) {
					executions.Add(1)
					<-release
					if tc.goexit {
						runtime.Goexit()
					}
					panic("secret")
				}
				for range 2 {
					go func() {
						var r result
						defer func() { r.panicValue = recover(); results <- r }()
						r.value, r.shared, r.err = g.DoShared(t.Context(), "k", fn)
					}()
					synctest.Wait()
				}
				close(release)
				for range 2 {
					r := <-results
					require.Zero(t, r.value)
					require.Equal(t, tc.wantPanic, r.panicValue)
					if tc.wantPanic == nil {
						require.True(t, r.shared)
						require.Same(t, tc.wantErr, r.err)
					}
				}
				g.Close()
				g.Wait()
				require.Equal(t, int64(1), executions.Load())
			})
		})
	}
}

func TestConcurrentForgetAndClose(t *testing.T) {
	t.Parallel()
	for _, lifecycle := range []bool{false, true} {
		name := "default"
		if lifecycle {
			name = "lifecycle"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testhelpers.VerifyNoLeaks(t)
			for range 20 {
				var opts []singleflight.Option
				if lifecycle {
					opts = append(opts, singleflight.WithLifecycle(t.Context()))
				}
				g := singleflight.New[int, int](opts...)
				var started, finished atomic.Int64
				fn := func(context.Context) (int, error) {
					started.Add(1)
					runtime.Gosched()
					finished.Add(1)
					return 7, nil
				}
				// Keep one forgotten execution alive across Close to exercise Wait's drain.
				entered, release, firstDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
				go func() {
					defer close(firstDone)
					_, _, _ = g.DoShared(t.Context(), -1, func(ctx context.Context) (int, error) {
						close(entered)
						<-release
						return fn(ctx)
					})
				}()
				<-entered
				g.Forget(-1)
				var wg sync.WaitGroup
				start := make(chan struct{})
				for worker := range 12 {
					wg.Go(func() {
						<-start
						for i := range 100 {
							key := (worker + i) % 4
							if i%3 == 0 {
								g.Forget(key)
							}
							v, _, err := g.DoShared(t.Context(), key, fn)
							if err == nil {
								if v != 7 {
									t.Errorf("value = %d", v)
								}
							} else if !errors.Is(err, singleflight.ErrClosed) {
								t.Errorf("DoShared = %v", err)
							}
							if worker == 0 && i == 50 {
								g.Close()
							}
						}
					})
				}
				close(start)
				wg.Wait()
				g.Close()
				close(release)
				g.Wait()
				<-firstDone
				require.Positive(t, started.Load())
				require.Equal(t, started.Load(), finished.Load())
				_, _, err := g.DoShared(t.Context(), 0, fn)
				require.ErrorIs(t, err, singleflight.ErrClosed)
			}
		})
	}
}

// Exercise both zero-counter admission and draining existing work with real
// concurrent callers: Wait must close admission before touching the WaitGroup.
func TestWaitClosesWhileCallsArrive(t *testing.T) {
	t.Parallel()
	for _, active := range []bool{false, true} {
		name := "empty"
		if active {
			name = "active"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testhelpers.VerifyNoLeaks(t)
			for range 20 {
				g := singleflight.New[int, int]()
				var started, finished, rejected atomic.Int64
				fn := func(context.Context) (int, error) {
					started.Add(1)
					runtime.Gosched()
					finished.Add(1)
					return 1, nil
				}
				entered, release, firstDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
				if active {
					go func() {
						defer close(firstDone)
						_, _ = g.Do(t.Context(), -1, func(ctx context.Context) (int, error) {
							close(entered)
							<-release
							return fn(ctx)
						})
					}()
					<-entered
				}
				start := make(chan struct{})
				var callers sync.WaitGroup
				for key := range 16 {
					callers.Go(func() {
						<-start
						for {
							v, err := g.Do(t.Context(), key, fn)
							if errors.Is(err, singleflight.ErrClosed) {
								rejected.Add(1)
								return
							}
							if err != nil || v != 1 {
								t.Errorf("Do = (%d, %v)", v, err)
								return
							}
						}
					})
				}
				var waiters sync.WaitGroup
				waited := make(chan struct{})
				for range 2 {
					waiters.Go(func() { <-start; g.Wait() })
				}
				go func() { waiters.Wait(); close(waited) }()
				close(start)
				callers.Wait()
				require.Equal(t, int64(16), rejected.Load())
				if active {
					select {
					case <-waited:
						t.Fatal("Wait returned before active work finished")
					default:
					}
					close(release)
					<-firstDone
				}
				<-waited
				require.Equal(t, started.Load(), finished.Load())
				_, err := g.Do(t.Context(), 0, fn)
				require.ErrorIs(t, err, singleflight.ErrClosed)
			}
		})
	}
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package cycle_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/cycle"
)

func TestGuard(t *testing.T) {
	t.Parallel()
	want := errors.New("cycle failed")
	calls := 0
	g := cycle.New(func(context.Context) error { calls++; return want })
	require.ErrorIs(t, g.Run(t.Context()), want)
	run := g.Managed()
	require.ErrorIs(t, g.Run(t.Context()), cycle.ErrManaged)
	require.ErrorIs(t, run(t.Context()), want)
	require.ErrorIs(t, g.Managed()(t.Context()), want)
	require.Equal(t, 3, calls)
	require.Panics(t, func() { cycle.New(nil) })
}

func TestOverlap(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		calls := 0
		g := cycle.New(func(context.Context) error { calls++; <-release; return nil })
		var wg sync.WaitGroup
		wg.Go(func() { _ = g.Run(t.Context()) })
		synctest.Wait()
		require.NoError(t, g.Run(t.Context()))
		run := g.Managed()
		wg.Go(func() { _ = run(t.Context()) })
		synctest.Wait()
		require.Equal(t, 1, calls)
		close(release)
		wg.Wait()
		require.NoError(t, run(t.Context()))
		require.Equal(t, 2, calls)
	})
}

func TestPanicReleasesGuard(t *testing.T) {
	t.Parallel()
	g := cycle.New(func(context.Context) error { panic("boom") })
	for range 2 {
		require.PanicsWithValue(t, "boom", func() { _ = g.Run(t.Context()) })
	}
}

func BenchmarkRun(b *testing.B) {
	g := cycle.New(func(context.Context) error { return nil })
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		_ = g.Run(ctx)
	}
}

func TestRegistration(t *testing.T) {
	t.Parallel()
	want := errors.New("cycle failed")
	for _, tc := range []struct {
		name    string
		mark    func(*cycle.Guard)
		managed bool
	}{
		{"unregistered", func(*cycle.Guard) {}, false},
		{"mark without wrapper", (*cycle.Guard).MarkManaged, true},
		{"managed convenience", func(g *cycle.Guard) { _ = g.Managed() }, true},
		{"rejected registration", func(g *cycle.Guard) {
			register := func(func(context.Context) error) error { return want }
			if register(g.Entry()) == nil {
				g.MarkManaged()
			}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			g := cycle.New(func(context.Context) error { calls++; return want })
			require.False(t, g.IsManaged())
			entry := g.Entry()
			require.False(t, g.IsManaged())
			require.ErrorIs(t, g.TryRun(t.Context()), want)
			tc.mark(g)
			require.Equal(t, tc.managed, g.IsManaged())
			if tc.managed {
				require.ErrorIs(t, g.Run(t.Context()), cycle.ErrManaged)
				require.Equal(t, 1, calls)
			} else {
				require.ErrorIs(t, g.Run(t.Context()), want)
				require.Equal(t, 2, calls)
			}
			before := calls
			require.ErrorIs(t, entry(t.Context()), want)
			require.ErrorIs(t, g.TryRun(t.Context()), want)
			g.MarkManaged()
			g.MarkManaged()
			require.True(t, g.IsManaged())
			require.ErrorIs(t, g.TryRun(t.Context()), want)
			require.Equal(t, before+3, calls)
		})
	}
}

func TestTryRunFunc(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		boundCalls := 0
		g := cycle.New(func(context.Context) error { boundCalls++; return nil })
		g.MarkManaged()
		entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
		want := errors.New("per-call error")
		fetched := 0
		go func() {
			done <- g.TryRunFunc(t.Context(), func(context.Context) error {
				fetched = 3
				close(entered)
				<-release
				return want
			})
		}()
		<-entered
		require.NoError(t, g.TryRun(t.Context()))
		require.NoError(t, g.Entry()(t.Context()))
		require.NoError(t, g.TryRunFunc(t.Context(), func(context.Context) error { t.Error("overlap executed"); return nil }))
		require.Zero(t, boundCalls)
		close(release)
		require.ErrorIs(t, <-done, want)
		require.Equal(t, 3, fetched)
		require.Panics(t, func() { _ = g.TryRunFunc(t.Context(), nil) })
		require.NoError(t, g.TryRun(t.Context()))
		require.Equal(t, 1, boundCalls)
	})
}

func TestConcurrentDrivers(t *testing.T) {
	t.Parallel()
	var active, maximum, calls atomic.Int64
	fn := func(context.Context) error {
		n := active.Add(1)
		for old := maximum.Load(); n > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, n) {
				break
			}
		}
		runtime.Gosched()
		calls.Add(1)
		active.Add(-1)
		return nil
	}
	g := cycle.New(fn)
	entry := g.Entry()
	for _, managed := range []bool{false, true} {
		if managed {
			g.MarkManaged()
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		for worker := range 24 {
			wg.Go(func() {
				<-start
				for range 200 {
					var err error
					switch worker % 4 {
					case 0:
						err = g.Run(t.Context())
					case 1:
						err = g.TryRun(t.Context())
					case 2:
						err = entry(t.Context())
					case 3:
						err = g.TryRunFunc(t.Context(), fn)
					}
					if managed && worker%4 == 0 {
						if !errors.Is(err, cycle.ErrManaged) {
							t.Errorf("Run = %v", err)
						}
					} else if err != nil {
						t.Errorf("driver = %v", err)
					}
				}
			})
		}
		close(start)
		wg.Wait()
	}
	require.Positive(t, calls.Load())
	require.Equal(t, int64(1), maximum.Load())
	require.Zero(t, active.Load())
}

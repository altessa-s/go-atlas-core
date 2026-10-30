// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
	"github.com/altessa-s/go-atlas-core/time/clock"
)

func newFakeClock() *clock.Fake {
	return clock.NewFake(time.Unix(1_700_000_000, 0))
}

func TestMemStatsCache_ReturnsSameDataWithinTTL(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	cache := concurrency.NewMemStatsCache(time.Second, clk)

	stats1 := cache.Get()
	require.NotNil(t, stats1)

	clk.Advance(time.Second - time.Nanosecond)
	require.Same(t, stats1, cache.Get(), "a snapshot within the TTL must be served from the cache")
}

func TestMemStatsCache_RefreshesAfterTTL(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	cache := concurrency.NewMemStatsCache(10*time.Millisecond, clk)

	stats1 := cache.Get()
	require.NotNil(t, stats1)

	clk.Advance(10 * time.Millisecond)
	stats2 := cache.Get()
	require.NotSame(t, stats1, stats2, "an expired snapshot must be refreshed")
	require.NotZero(t, stats2.Sys)
}

func TestMemStatsCache_Invalidate(t *testing.T) {
	t.Parallel()
	cache := concurrency.NewMemStatsCache(time.Hour, newFakeClock())

	stats1 := cache.Get()
	require.Same(t, stats1, cache.Get())

	cache.Invalidate()
	require.NotSame(t, stats1, cache.Get(), "Invalidate must force a refresh")
}

func TestMemStatsCache_ZeroTTLDisablesCaching(t *testing.T) {
	t.Parallel()
	cache := concurrency.NewMemStatsCache(0, newFakeClock())

	require.NotSame(t, cache.Get(), cache.Get())
}

func TestNewMemStatsCache_Defaults(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	cache := concurrency.NewMemStatsCache(-1, clk)

	stats1 := cache.Get()
	clk.Advance(concurrency.DefaultMemStatsCacheTTL - time.Nanosecond)
	require.Same(t, stats1, cache.Get(), "a negative TTL must fall back to DefaultMemStatsCacheTTL")
	clk.Advance(time.Nanosecond)
	require.NotSame(t, stats1, cache.Get())

	require.NotNil(t, concurrency.NewMemStatsCache(time.Second, nil).Get(), "a nil clock must fall back to the real clock")
}

func TestMemStatsCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	cache := concurrency.NewMemStatsCache(50*time.Millisecond, clock.Real())

	const numGoroutines = 100
	var wg sync.WaitGroup
	results := make(chan *runtime.MemStats, numGoroutines)

	for range numGoroutines {
		wg.Go(func() {
			results <- cache.Get()
		})
	}

	wg.Wait()
	close(results)

	count := 0
	for stats := range results {
		require.NotNil(t, stats, "got nil stats from concurrent call")
		count++
	}
	require.Equal(t, numGoroutines, count)
}

func TestMemStatsCache_AsMemorySource(t *testing.T) {
	t.Parallel()
	cache := concurrency.NewMemStatsCache(time.Hour, newFakeClock())

	limit := concurrency.MemoryAwareConcurrency(100, 500, 1000, concurrency.WithAvailableMemory(cache.RuntimeHeadroom))
	require.GreaterOrEqual(t, limit(), concurrency.MinConcurrencyLimit)
}

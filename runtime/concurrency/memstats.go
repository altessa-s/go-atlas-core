// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package concurrency

import (
	"math"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/altessa-s/go-atlas-core/time/clock"
	"github.com/altessa-s/go-atlas-core/types/nilcheck"
)

// DefaultMemStatsCacheTTL is the default time-to-live for the cached
// runtime.MemStats snapshot used by [MemoryAwareConcurrency] and
// [AdaptiveConcurrency]. It balances data freshness against the cost
// of runtime.ReadMemStats, which triggers a stop-the-world pause.
// Use [NewMemStatsCache] for a cache with another TTL.
const DefaultMemStatsCacheTTL = time.Second

// MemStatsCache caches runtime.MemStats snapshots with TTL-based
// invalidation. This reduces the frequency of runtime.ReadMemStats calls,
// which cause stop-the-world pauses of 50-200μs each.
//
// Pass [MemStatsCache.RuntimeHeadroom] to [WithAvailableMemory] to drive
// [MemoryAwareConcurrency] or [AdaptiveConcurrency] from a dedicated cache;
// without it they share an internal cache with [DefaultMemStatsCacheTTL].
// A MemStatsCache is safe for concurrent use.
type MemStatsCache struct {
	stats       atomic.Pointer[runtime.MemStats]
	lastUpdated atomic.Int64 // Unix nanoseconds; 0 means stale
	mu          sync.Mutex   // Serializes refreshes
	ttl         time.Duration
	clock       clock.Clock
}

// defaultMemStatsCache backs the default [MemorySource]. It is never
// reconfigured: callers that need another TTL or clock build their own
// [MemStatsCache].
var defaultMemStatsCache = NewMemStatsCache(DefaultMemStatsCacheTTL, clock.Real())

// NewMemStatsCache returns a cache whose snapshots stay valid for ttl as
// measured by clk. Shorter durations provide fresher data at the expense of
// more frequent stop-the-world pauses; longer durations reduce pauses but
// may cause concurrency decisions to use stale data.
//
// Recommended values:
//   - 100ms: latency-sensitive applications needing fresh memory readings
//   - 1s (default): good balance for most applications
//   - 5s: applications where memory pressure changes slowly
//
// A negative ttl is replaced with [DefaultMemStatsCacheTTL]; a zero ttl
// disables caching. A nil clk is replaced with clock.Real().
func NewMemStatsCache(ttl time.Duration, clk clock.Clock) *MemStatsCache {
	if ttl < 0 {
		ttl = DefaultMemStatsCacheTTL
	}
	if nilcheck.IsNil(clk) {
		clk = clock.Real()
	}
	return &MemStatsCache{ttl: ttl, clock: clk}
}

// Get returns the cached memory statistics, refreshing them when stale.
// The returned pointer MUST be treated as read-only; it points directly into
// the cache to avoid copying the 4.8 KB runtime.MemStats struct on every call.
//
// The cache uses a TTL-based invalidation strategy:
//   - If the cache is fresh (within TTL), it returns the cached values immediately
//   - If the cache is stale, one goroutine refreshes while others get stale data
//   - This prevents a thundering herd on cache expiration
func (c *MemStatsCache) Get() *runtime.MemStats {
	// Fast path: get current pointer
	current := c.stats.Load()

	// Check if cache is still fresh and we have data
	if current != nil && c.fresh() {
		return current
	}

	// Try to acquire the update lock
	// If another goroutine is updating, return stale data rather than wait
	if !c.mu.TryLock() {
		// Another goroutine is refreshing, return current (possibly stale) data
		if current != nil {
			return current
		}
		// If we have no data yet, we must wait for the lock
		c.mu.Lock()
	}
	defer c.mu.Unlock()

	// Double-check after acquiring lock (another goroutine might have updated)
	current = c.stats.Load()
	if current != nil && c.fresh() {
		return current
	}

	// Refresh the cache
	// Allocate new struct to avoid modifying the one pointed to by atomic pointer
	newStats := new(runtime.MemStats)
	runtime.ReadMemStats(newStats)

	c.stats.Store(newStats)
	c.lastUpdated.Store(c.clock.Now().UnixNano())

	return newStats
}

// fresh reports whether the last refresh is still within the TTL.
func (c *MemStatsCache) fresh() bool {
	lastUpdated := c.lastUpdated.Load()
	return lastUpdated > 0 && c.clock.Now().UnixNano()-lastUpdated < int64(c.ttl)
}

// Invalidate marks the cached memory statistics as stale, causing the next
// [MemStatsCache.Get] to call runtime.ReadMemStats. Use this after large
// allocations or deallocations where fresh data is critical.
func (c *MemStatsCache) Invalidate() {
	c.lastUpdated.Store(0)
}

// RuntimeHeadroom is a [MemorySource] that measures the remaining Go runtime
// memory budget set through GOMEMLIMIT/debug.SetMemoryLimit, reading memory
// statistics through c. Runtime-managed memory is Sys minus HeapReleased, as
// defined by SetMemoryLimit; it is not machine free RAM.
func (c *MemStatsCache) RuntimeHeadroom() (uint64, bool) {
	limit := debug.SetMemoryLimit(-1)
	if limit == math.MaxInt64 {
		return 0, false
	}
	return memoryHeadroom(c.Get(), limit)
}

func memoryHeadroom(stats *runtime.MemStats, limit int64) (uint64, bool) {
	if limit < 0 || limit == math.MaxInt64 {
		return 0, false
	}
	used := stats.Sys - min(stats.Sys, stats.HeapReleased)
	if used >= uint64(limit) {
		return 0, true
	}
	return uint64(limit) - used, true
}

func getAvailableMemoryMB(source MemorySource) (uint64, bool) {
	available, known := source()
	return available / BytesToMB / BytesToMB, known
}

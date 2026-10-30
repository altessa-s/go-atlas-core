// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package signals

import (
	"cmp"
	"slices"
	"sync"
)

// priorityBucket represents a bucket of handlers for a specific priority level
// This enables O(1) priority-based execution instead of O(n²) sorting
type priorityBucket struct {
	priority Priority
	handlers []handlerEntry
}

// priorityQueue implements an optimized priority queue using buckets.
//
// Predefined priorities land in fixed buckets; every other value gets its own
// bucket in custom, kept sorted by descending priority. Each bucket preserves
// registration order, so merging the two descending bucket lists yields a
// stable descending sequence without sorting.
type priorityQueue struct {
	buckets [PriorityBucketCount]*priorityBucket
	// custom holds one bucket per custom (non-predefined) priority, sorted by
	// descending priority. Its priorities never collide with buckets because
	// addHandler routes exact matches into the predefined buckets first.
	custom []*priorityBucket
	mutex  sync.RWMutex
}

// newPriorityQueue creates an optimized priority queue
func newPriorityQueue() *priorityQueue {
	pq := &priorityQueue{}
	// Pre-allocate buckets for common priority levels
	priorities := []Priority{PriorityHighest, PriorityHigh, PriorityNormal, PriorityLow, PriorityLowest}

	for i, priority := range priorities {
		if i < len(pq.buckets) {
			pq.buckets[i] = &priorityBucket{
				priority: priority,
				handlers: make([]handlerEntry, 0, 4), // Small initial capacity
			}
		}
	}

	return pq
}

// addHandler adds a handler to the appropriate priority bucket
func (pq *priorityQueue) addHandler(entry handlerEntry) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	// Find the appropriate bucket
	for _, bucket := range pq.buckets {
		if bucket != nil && bucket.priority == entry.priority {
			bucket.handlers = append(bucket.handlers, entry)
			return
		}
	}

	// Custom priority with no matching predefined bucket: find or create its
	// own bucket, keeping custom sorted by descending priority.
	i, found := slices.BinarySearchFunc(pq.custom, entry.priority, func(b *priorityBucket, p Priority) int {
		return cmp.Compare(p, b.priority)
	})
	if !found {
		pq.custom = slices.Insert(pq.custom, i, &priorityBucket{
			priority: entry.priority,
			handlers: make([]handlerEntry, 0, 4), // Same initial capacity as the predefined buckets
		})
	}
	pq.custom[i].handlers = append(pq.custom[i].handlers, entry)
}

// getHandlersInOrder returns all handlers sorted by priority (high to low)
func (pq *priorityQueue) getHandlersInOrder() []handlerEntry {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()

	totalHandlers := 0
	for _, bucket := range pq.buckets {
		if bucket != nil {
			totalHandlers += len(bucket.handlers)
		}
	}
	for _, bucket := range pq.custom {
		totalHandlers += len(bucket.handlers)
	}

	if totalHandlers == 0 {
		return nil
	}

	// Pre-allocate result slice with exact capacity
	result := make([]handlerEntry, 0, totalHandlers)

	// Merge the predefined and custom bucket lists, both descending by
	// priority, from highest to lowest.
	custom := pq.custom
	for _, bucket := range pq.buckets {
		if bucket == nil {
			continue
		}
		for len(custom) > 0 && custom[0].priority > bucket.priority {
			result = append(result, custom[0].handlers...)
			custom = custom[1:]
		}
		result = append(result, bucket.handlers...)
	}
	for _, bucket := range custom {
		result = append(result, bucket.handlers...)
	}

	return result
}

// insertionSortThreshold is the handler-count cutoff below which sortHandlers
// uses the in-place insertion sort. For small slices insertion sort beats
// counting sort on wall-clock because it has zero allocations and extremely
// cache-friendly access; counting sort only catches up once n is large
// enough to amortize its two buffer allocations.
//
// The value is empirical: see BenchmarkSortHandlers_Dispatch vs
// BenchmarkSortHandlersByPriority_Insertion / _Counting. On Apple M4 Pro
// with Go 1.25, insertion sort wins below ~32 entries and counting sort
// dominates above it.
const insertionSortThreshold = 32

// countingSortMaxRange is the absolute cap on the distinct priority range the
// counting sort will handle. If handlers use priorities spread across a
// wider range, counting sort would allocate a wastefully large counts slice,
// so the dispatcher falls back to insertion sort. This ceiling bounds the
// worst-case memory regardless of input.
const countingSortMaxRange = 1024

// sortHandlers sorts handlers by priority in descending order (higher priority
// first), preserving registration order among handlers of equal priority.
//
// It dispatches between two stable algorithms:
//
//   - For slices with at most [insertionSortThreshold] entries, the in-place
//     insertion sort in [Signal.sortHandlersByPriority] runs in O(n²) but
//     with zero allocations and excellent cache locality, which beats
//     counting sort at small n.
//   - For larger slices, [Signal.sortHandlersByPriorityCounting] runs a
//     stable counting sort in O(n+k) where k is the observed priority
//     range. Since the documented Priority constants occupy [1..100], k is
//     tiny in practice and counting sort dominates asymptotically.
//
// If the observed priority range is pathologically wide — a caller using
// custom values that exceed [countingSortMaxRange] distinct levels or are
// more than 8× sparser than n — counting sort would allocate an unreasonably
// large counts slice, so the dispatcher falls back to insertion sort. The
// guardrail keeps memory bounded regardless of caller behavior.
func (s *Signal) sortHandlers(handlers []handlerEntry) {
	if len(handlers) <= insertionSortThreshold {
		s.sortHandlersByPriority(handlers)
		return
	}
	if !s.sortHandlersByPriorityCounting(handlers) {
		s.sortHandlersByPriority(handlers)
	}
}

// sortHandlersByPriority sorts handlers by priority in descending order
// (higher priority first) using a stable in-place insertion sort. It is the
// best algorithm for small handler counts and serves as the fallback for
// [Signal.sortHandlers] when counting sort is not applicable.
func (s *Signal) sortHandlersByPriority(handlers []handlerEntry) {
	for i := 1; i < len(handlers); i++ {
		current := handlers[i]
		j := i - 1

		// Move handlers with lower priority to the right.
		for j >= 0 && handlers[j].priority < current.priority {
			handlers[j+1] = handlers[j]
			j--
		}
		handlers[j+1] = current
	}
}

// sortHandlersByPriorityCounting performs a stable counting sort over
// handlers keyed by priority, producing descending order (higher priority
// first). It returns false — without modifying handlers — when the priority
// range is too wide to counting-sort efficiently, so the caller can fall
// back to a comparison sort.
//
// Complexity is O(n+k) where k is the observed priority range
// (max-min+1). The function allocates a counts slice of length k and an
// output buffer of length n, then copies the output back in place. Both
// allocations are local to the call; no state is retained between
// invocations.
//
// Stability: the scatter pass walks handlers left-to-right and increments
// the per-priority cursor as it writes, so entries that share a priority
// retain their original relative order. The rest of the dispatch pipeline
// relies on this to fire handlers in registration order within a priority
// level.
func (s *Signal) sortHandlersByPriorityCounting(handlers []handlerEntry) bool {
	n := len(handlers)
	if n <= 1 {
		return true
	}

	// Single pass: determine the priority range.
	minP, maxP := handlers[0].priority, handlers[0].priority
	for _, h := range handlers[1:] {
		if h.priority < minP {
			minP = h.priority
		}
		if h.priority > maxP {
			maxP = h.priority
		}
	}

	// Guard against pathologically sparse or wide ranges. A range wider
	// than [countingSortMaxRange] would waste memory outright; a range
	// wider than 8*n means fewer than one in eight buckets is used,
	// which is the empirical point past which insertion sort is faster
	// than counting sort's buffer allocation + scatter + copy-back cost.
	rng := int(maxP-minP) + 1
	if rng <= 0 || rng > countingSortMaxRange || rng > 8*n {
		return false
	}

	// Count occurrences per priority level.
	counts := make([]int, rng)
	for _, h := range handlers {
		counts[int(h.priority-minP)]++
	}

	// Convert counts to descending starting positions in the output.
	// After this loop, counts[i] holds the index in `out` where the
	// first handler with priority (minP+i) should be written; walking
	// the priority axis from high to low places the highest priorities
	// at the front.
	var total int
	for i := rng - 1; i >= 0; i-- {
		c := counts[i]
		counts[i] = total
		total += c
	}

	// Scatter into a fresh output buffer, preserving input order for
	// equal priorities (stability).
	out := make([]handlerEntry, n)
	for _, h := range handlers {
		idx := int(h.priority - minP)
		out[counts[idx]] = h
		counts[idx]++
	}

	copy(handlers, out)
	return true
}

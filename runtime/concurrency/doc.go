// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

// Package concurrency provides utilities for managing concurrent execution.
// It includes functions for calculating optimal concurrency limits based on
// runtime conditions (CPU, memory, load) and a concurrent batch processor
// for parallelizing work across collections.
//
// # Concurrency Calculation
//
// The package offers several strategies for determining optimal concurrency:
//   - Environment-based: predefined limits for common scenarios (CPU-bound, IO-bound, etc.)
//   - Memory-aware: adjusts limits based on remaining configured memory budget
//   - Load-aware: adjusts limits based on system load averages
//   - Adaptive: combines multiple factors for complex resource management
//
// Memory-aware and adaptive limits read runtime.MemStats through a TTL cache.
// Inject a [MemStatsCache] with WithAvailableMemory(cache.RuntimeHeadroom) to
// choose its TTL and clock; otherwise an internal cache with
// [DefaultMemStatsCacheTTL] is shared.
//
// # Batch Processing
//
// The batch processor allows concurrent execution of functions over slices with
// configurable concurrency limits and error handling strategies. It supports
// both fixed limits and dynamic calculation via ConcurrencyLimitFunc.
//
// Example Batch Usage:
//
//	items := []string{"a", "b", "c"}
//
//	opts := []concurrency.Option[string]{
//	    concurrency.WithLimitFunc[string](concurrency.MemoryAwareConcurrency(100, 500, 1000)),
//	    concurrency.WithStopOnError[string](),
//	    concurrency.WithOnSuccess[string](func(item string) {
//	        fmt.Printf("Processed: %s\n", item)
//	    }),
//	    concurrency.WithOnError[string](func(item string, err error) {
//	        fmt.Printf("Failed: %s, error: %v\n", item, err)
//	    }),
//	}
//
//	// Simple parallel processing
//	err := concurrency.Process(ctx, items, func(ctx context.Context, item string) error {
//	    return processItem(ctx, item)
//	}, opts...)
//
//	// Parallel transformation and collection
//	results, err := concurrency.ProcessCollect(ctx, items, func(ctx context.Context, item string) (int, error) {
//	    return len(item), nil
//	}, opts...)
//
// # Ordered Failover
//
// [First] tries named sources one after another and returns the first value an
// accept policy admits. Each skipped source is recorded as a [SourceError]; when
// every source is skipped the error joins [ErrAllFailed] with all causes. A done
// context is returned as ctx.Err() and is never counted as a source failure;
// [Canceled] tells such a cancellation from an operational failure.
//
//	res, err := concurrency.First(ctx, []concurrency.Source[Snapshot]{
//	    {Name: "primary", Fetch: primary.Fetch},
//	    {Name: "mirror", Fetch: mirror.Fetch},
//	}, func(source string, s Snapshot) error {
//	    return validate(s)
//	})
//	if concurrency.Canceled(ctx, err) {
//	    return err
//	}
package concurrency

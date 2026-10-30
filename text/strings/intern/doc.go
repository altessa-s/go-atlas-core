// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

// Package intern deduplicates repeated string values so only one copy is
// retained in memory.
//
// # Interner
//
// [Interner] is a lock-free, concurrency-safe interner with a two-tier cache
// (a hot cache of atomic slots plus a cold [sync.Map]) and LFU eviction with
// LRU tie-breaking once it exceeds its maximum size. [Interner.Stats] reports
// hit, miss, and eviction counters.
//
// # Default interner
//
// Every behavior is available on an explicitly constructed [Interner] from
// [New]. The package-level functions ([String], [LowerString],
// [JoinString], ...) are thin wrappers over the process-wide [Default]
// interner; [ResetDefault] clears it, which is mainly useful in tests.
//
// # Options
//
// [WithClock] injects the time source used for access timestamps (default:
// the real clock), so eviction order is deterministic under a fake clock.
//
// # Usage
//
//	in := intern.New(4096)
//	canonical := in.String("/api/users")
//
//	// Or the process-wide default
//	canonical = intern.String("/api/users")
//	stats := intern.Default().Stats()
//	fmt.Printf("hit rate: %.1f%%\n", stats.HitRate()*100)
package intern

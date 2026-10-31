// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package intern_test

import (
	"fmt"
	"testing"

	"github.com/altessa-s/go-atlas-core/text/strings/intern"
)

// benchColdKeys returns n distinct pre-interned keys. With only
// [intern.HotCacheSlots] hot slots, rotating over thousands of keys
// keeps lookups on the cold-cache path.
func benchColdKeys(si *intern.Interner, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("interner-bench-key-%05d", i)
		si.String(keys[i])
	}
	return keys
}

// BenchmarkInterner_ColdHit pins the sequential cold-cache hit cost.
func BenchmarkInterner_ColdHit(b *testing.B) {
	si := intern.New(intern.DefaultMaxSize)
	keys := benchColdKeys(si, 4096)

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_ = si.String(keys[i%len(keys)])
		i++
	}
}

// BenchmarkInterner_ColdHitParallel exercises the same cold-cache path from
// concurrent goroutines.
func BenchmarkInterner_ColdHitParallel(b *testing.B) {
	si := intern.New(intern.DefaultMaxSize)
	keys := benchColdKeys(si, 4096)

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = si.String(keys[i%len(keys)])
			i++
		}
	})
}

// BenchmarkInterner_HotHit measures lookups of a handful of keys accessed
// often enough to be promoted to the lock-free hot cache.
func BenchmarkInterner_HotHit(b *testing.B) {
	si := intern.New(intern.DefaultMaxSize)
	keys := []string{"GET", "POST", "content-type", "authorization"}
	for range intern.PromotionCheckInterval * 2 {
		for _, k := range keys {
			si.String(k)
		}
	}

	b.Run("Sequential", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			_ = si.String(keys[i%len(keys)])
			i++
		}
	})
	b.Run("Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = si.String(keys[i%len(keys)])
				i++
			}
		})
	})
}

// BenchmarkInterner_MissWithEviction measures interning a stream of new
// strings into a bounded interner, so inserts keep triggering eviction.
func BenchmarkInterner_MissWithEviction(b *testing.B) {
	const maxSize = 1024
	si := intern.New(maxSize)
	keys := make([]string, 1<<15)
	for i := range keys {
		keys[i] = fmt.Sprintf("stream-key-%06d", i)
	}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_ = si.String(keys[i%len(keys)])
		i++
	}
}

// BenchmarkInterner_Derived measures the helpers that build a string and
// intern the result, on keys that are already interned.
func BenchmarkInterner_Derived(b *testing.B) {
	si := intern.New(intern.DefaultMaxSize)
	parts := []string{"service", "region", "shard-07"}

	b.Run("LowerString", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if si.LowerString("Content-Type") != "content-type" {
				b.Fatal("unexpected lowercase")
			}
		}
	})
	b.Run("JoinString", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if si.JoinString(parts, ".") != "service.region.shard-07" {
				b.Fatal("unexpected join")
			}
		}
	})
	b.Run("StringSlice", func(b *testing.B) {
		labels := make([]string, 16)
		for i := range labels {
			labels[i] = fmt.Sprintf("label-%d", i%4)
		}
		b.ReportAllocs()
		for b.Loop() {
			if len(si.StringSlice(labels)) != len(labels) {
				b.Fatal("unexpected length")
			}
		}
	})
}

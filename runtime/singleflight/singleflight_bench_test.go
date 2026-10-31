// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package singleflight_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/altessa-s/go-atlas-core/runtime/singleflight"
)

func BenchmarkDo(b *testing.B) {
	g := singleflight.New[string, int]()
	ctx := b.Context()
	fn := func(context.Context) (int, error) { return 1, nil }
	b.ReportAllocs()
	for b.Loop() {
		_, _ = g.Do(ctx, "key", fn)
	}
}

// BenchmarkDo_Parallel measures concurrent callers on one hot key, where
// calls overlap and get coalesced, and on distinct keys, where every call
// runs its own flight.
func BenchmarkDo_Parallel(b *testing.B) {
	fn := func(context.Context) (int, error) { return 1, nil }

	b.Run("SameKey", func(b *testing.B) {
		g := singleflight.New[string, int]()
		ctx := b.Context()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if v, err := g.Do(ctx, "hot", fn); err != nil || v != 1 {
					b.Errorf("Do = (%d, %v)", v, err)
					return
				}
			}
		})
	})
	b.Run("DistinctKeys", func(b *testing.B) {
		g := singleflight.New[int, int]()
		ctx := b.Context()
		var next atomic.Int64
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			key := int(next.Add(1))
			for pb.Next() {
				if v, err := g.Do(ctx, key, fn); err != nil || v != 1 {
					b.Errorf("Do = (%d, %v)", v, err)
					return
				}
			}
		})
	})
}

// BenchmarkDoShared covers the shared-result API with and without lifecycle
// ownership, including one hot key and independent keys under contention.
func BenchmarkDoShared(b *testing.B) {
	for _, lifecycle := range []bool{false, true} {
		name := "Default"
		if lifecycle {
			name = "WithLifecycle"
		}
		b.Run(name, func(b *testing.B) { benchmarkCalls(b, true, lifecycle) })
	}
}

func BenchmarkDoWithLifecycle(b *testing.B) { benchmarkCalls(b, false, true) }

func benchmarkCalls(b *testing.B, shared, lifecycle bool) {
	b.Helper()
	for _, mode := range []string{"SingleKey", "ParallelSameKey", "ParallelDistinctKeys"} {
		b.Run(mode, func(b *testing.B) {
			ctx := b.Context()
			var opts []singleflight.Option
			if lifecycle {
				opts = append(opts, singleflight.WithLifecycle(ctx))
			}
			g := singleflight.New[int, int](opts...)
			b.Cleanup(g.Wait)
			fn := func(context.Context) (int, error) { return 1, nil }
			call := func(key int) {
				var value int
				var err error
				if shared {
					value, _, err = g.DoShared(ctx, key, fn)
				} else {
					value, err = g.Do(ctx, key, fn)
				}
				if err != nil || value != 1 {
					b.Errorf("call = (%d, %v)", value, err)
				}
			}
			b.ReportAllocs()
			if mode == "SingleKey" {
				for b.Loop() {
					call(0)
				}
				return
			}
			var next atomic.Int64
			b.RunParallel(func(pb *testing.PB) {
				key := 0
				if mode == "ParallelDistinctKeys" {
					key = int(next.Add(1))
				}
				for pb.Next() {
					call(key)
				}
			})
		})
	}
}

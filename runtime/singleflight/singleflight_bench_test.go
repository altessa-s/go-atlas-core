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

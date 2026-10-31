// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package context_test

import (
	"context"
	"testing"
	"time"

	corecontext "github.com/altessa-s/go-atlas-core/context"
)

func BenchmarkApplyTimeout(b *testing.B) {
	ctx := b.Context() // no deadline => timer context is created

	b.ReportAllocs()
	for b.Loop() {
		tctx, cancel := corecontext.ApplyTimeout(ctx, time.Second)
		cancel()
		_ = tctx
	}
}

func BenchmarkApplyTimeout_ExistingDeadline(b *testing.B) {
	ctx, cancel := context.WithTimeout(b.Context(), time.Hour)
	defer cancel()

	b.ReportAllocs()
	for b.Loop() {
		tctx, tcancel := corecontext.ApplyTimeout(ctx, time.Second)
		tcancel()
		_ = tctx
	}
}

// BenchmarkWithMaxTimeout measures capping a request context: the sooner
// incoming deadline is kept as is, a later one gets a new timer context.
func BenchmarkWithMaxTimeout(b *testing.B) {
	b.Run("SoonerDeadline", func(b *testing.B) {
		ctx, cancel := context.WithTimeout(b.Context(), time.Second)
		defer cancel()
		b.ReportAllocs()
		for b.Loop() {
			tctx, tcancel := corecontext.WithMaxTimeout(ctx, time.Hour)
			tcancel()
			if tctx != ctx {
				b.Fatal("context replaced despite sooner deadline")
			}
		}
	})
	b.Run("LaterDeadline", func(b *testing.B) {
		ctx, cancel := context.WithTimeout(b.Context(), time.Hour)
		defer cancel()
		b.ReportAllocs()
		for b.Loop() {
			tctx, tcancel := corecontext.WithMaxTimeout(ctx, time.Second)
			tcancel()
			if tctx == ctx {
				b.Fatal("deadline not capped")
			}
		}
	})
}

func BenchmarkWithDefault(b *testing.B) {
	ctx := b.Context() // no deadline => timer context is created

	b.ReportAllocs()
	for b.Loop() {
		tctx, cancel := corecontext.WithDefault(ctx, time.Second)
		cancel()
		_ = tctx
	}
}

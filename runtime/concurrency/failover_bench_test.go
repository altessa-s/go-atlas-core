// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"context"
	"errors"
	"testing"

	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
)

var errBenchFetch = errors.New("bench fetch failed")

func benchSources(failing int) []concurrency.Source[int] {
	out := make([]concurrency.Source[int], 0, failing+1)
	for range failing {
		out = append(out, concurrency.Source[int]{Name: "down", Fetch: func(context.Context) (int, error) { return 0, errBenchFetch }})
	}
	return append(out, concurrency.Source[int]{Name: "up", Fetch: func(context.Context) (int, error) { return 1, nil }})
}

func BenchmarkFirstImmediate(b *testing.B) {
	ctx := b.Context()
	srcs := benchSources(0)
	for b.Loop() {
		_, _ = concurrency.First(ctx, srcs, nil)
	}
}

func BenchmarkFirstAfterFailures(b *testing.B) {
	ctx := b.Context()
	srcs := benchSources(3)
	for b.Loop() {
		_, _ = concurrency.First(ctx, srcs, nil)
	}
}

func BenchmarkFirstAllFailed(b *testing.B) {
	ctx := b.Context()
	srcs := benchSources(4)[:4]
	for b.Loop() {
		_, _ = concurrency.First(ctx, srcs, nil)
	}
}

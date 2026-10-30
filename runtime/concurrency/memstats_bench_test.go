// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
	"github.com/altessa-s/go-atlas-core/time/clock"
)

func BenchmarkMemStatsCacheGet(b *testing.B) {
	cache := concurrency.NewMemStatsCache(time.Second, clock.Real())

	for b.Loop() {
		_ = cache.Get()
	}
}

func BenchmarkRuntimeReadMemStats(b *testing.B) {
	var m runtime.MemStats
	for b.Loop() {
		runtime.ReadMemStats(&m)
	}
}

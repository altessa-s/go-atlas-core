// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package maps_test

import (
	"testing"

	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

func BenchmarkPoolReuse(b *testing.B) {
	pool := coremaps.NewPool[int, int](64)
	b.ReportAllocs()
	for b.Loop() {
		m := pool.Get()
		(*m)[1] = 1
		pool.Put(m)
	}
}

func BenchmarkPoolGetWithCapacity(b *testing.B) {
	pool := coremaps.NewPool[int, int](64)
	b.ReportAllocs()
	for b.Loop() {
		m := pool.GetWithCapacity(32)
		(*m)[1] = 1
		pool.Put(m)
	}
}

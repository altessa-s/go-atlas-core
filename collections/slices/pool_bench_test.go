// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package slices_test

import (
	"testing"

	coreslices "github.com/altessa-s/go-atlas-core/collections/slices"
)

func BenchmarkPoolReuse(b *testing.B) {
	pool := coreslices.NewPool[*int](64)
	value := new(1)
	b.ReportAllocs()
	for b.Loop() {
		buf := pool.Get()
		*buf = append(*buf, value)
		pool.Put(buf)
	}
}

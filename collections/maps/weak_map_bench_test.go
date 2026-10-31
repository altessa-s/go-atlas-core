// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package maps_test

import (
	"fmt"
	"runtime"
	"testing"

	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

// weakBenchValue is the cached value type; a struct keeps the value on the
// heap so the weak pointer has something to track.
type weakBenchValue struct {
	payload [64]byte
}

// newWeakBenchMap returns a WeakMap holding size live entries and the strong
// references that keep them alive for the benchmark's duration.
func newWeakBenchMap(size int) (*coremaps.WeakMap[string, weakBenchValue], []string, []*weakBenchValue) {
	m := coremaps.NewWeakMap[string, weakBenchValue]()
	keys := make([]string, size)
	vals := make([]*weakBenchValue, size)
	for i := range size {
		keys[i] = fmt.Sprintf("key-%d", i)
		vals[i] = new(weakBenchValue)
		m.Set(keys[i], vals[i])
	}
	return m, keys, vals
}

// BenchmarkWeakMap_Get measures a cache lookup of a live entry, sequentially
// and under read contention.
func BenchmarkWeakMap_Get(b *testing.B) {
	const size = 1024
	m, keys, vals := newWeakBenchMap(size)

	b.Run("Hit", func(b *testing.B) {
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			if _, ok := m.Get(keys[i%size]); !ok {
				b.Fatal("live entry missing")
			}
			i++
		}
	})
	b.Run("Miss", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, ok := m.Get("absent"); ok {
				b.Fatal("unexpected hit")
			}
		}
	})
	b.Run("HitParallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if _, ok := m.Get(keys[i%size]); !ok {
					b.Error("live entry missing")
					return
				}
				i++
			}
		})
	})
	runtime.KeepAlive(vals)
}

// BenchmarkWeakMap_Set measures caching a value under an existing key:
//   - Fresh caches a freshly built value, which registers a GC cleanup for it
//     and stops the replaced value's; the replaced values become garbage as
//     they would in a cache.
//   - ReplaceLive rotates the live values across the keys, so every Set
//     replaces one live value with another; stopped cleanups must not pile up
//     on the values that stay alive.
//   - SameLive re-sets the value a key already holds.
func BenchmarkWeakMap_Set(b *testing.B) {
	const size = 1024
	b.Run("Fresh", func(b *testing.B) {
		m, keys, vals := newWeakBenchMap(size)
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			m.Set(keys[i%size], new(weakBenchValue))
			i++
		}
		runtime.KeepAlive(vals)
	})
	b.Run("ReplaceLive", func(b *testing.B) {
		m, keys, vals := newWeakBenchMap(size)
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			// Round r = i/size gives key k the value k+r+1, never the one it holds.
			m.Set(keys[i%size], vals[(i%size+i/size+1)%size])
			i++
		}
		runtime.KeepAlive(vals)
	})
	b.Run("SameLive", func(b *testing.B) {
		m, keys, vals := newWeakBenchMap(size)
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			m.Set(keys[i%size], vals[i%size])
			i++
		}
		runtime.KeepAlive(vals)
	})
}

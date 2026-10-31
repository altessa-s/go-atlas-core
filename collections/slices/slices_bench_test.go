// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package slices_test

import (
	"fmt"
	"testing"

	coreslices "github.com/altessa-s/go-atlas-core/collections/slices"
)

func BenchmarkDeduplicate(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		input := make([]int, size)
		for i := range size {
			input[i] = i % (size / 2) // Guarantee 50% duplicates
		}

		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			for b.Loop() {
				coreslices.Deduplicate(input)
			}
		})
	}
}

// BenchmarkDeduplicate_Unique measures the no-duplicate path, where the seen
// map has to hold every element and the original slice is returned.
func BenchmarkDeduplicate_Unique(b *testing.B) {
	for _, size := range []int{100, 1000, 10000, 100000} {
		input := make([]int, size)
		for i := range size {
			input[i] = i
		}

		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			for b.Loop() {
				coreslices.Deduplicate(input)
			}
		})
	}
}

// BenchmarkDeduplicate_FewUnique measures a large input made of 16 distinct
// values, where the seen map stays tiny however long the input is.
func BenchmarkDeduplicate_FewUnique(b *testing.B) {
	for _, size := range []int{1000, 100000} {
		input := make([]int, size)
		for i := range size {
			input[i] = i % 16
		}

		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			for b.Loop() {
				coreslices.Deduplicate(input)
			}
		})
	}
}

func BenchmarkFilterParallel(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}
	for _, size := range sizes {
		input := make([]int, size)
		for i := range size {
			input[i] = i
		}
		predicate := func(n int) bool { return n%2 == 0 }

		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			for b.Loop() {
				coreslices.FilterParallel(input, predicate)
			}
		})
	}
}

func BenchmarkMapParallel(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}
	for _, size := range sizes {
		input := make([]int, size)
		for i := range size {
			input[i] = i
		}
		fn := func(n int) int { return n * 2 }

		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			for b.Loop() {
				coreslices.MapParallel(input, fn)
			}
		})
	}
}

// benchSizes are the collection sizes the sequential helpers are measured
// at: a typical request-sized slice and a batch-sized one.
var benchSizes = []int{100, 10000}

// benchInts returns 0..size-1.
func benchInts(size int) []int {
	input := make([]int, size)
	for i := range size {
		input[i] = i
	}
	return input
}

func BenchmarkTo(b *testing.B) {
	for _, size := range benchSizes {
		input := benchInts(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				out := coreslices.To(input, func(n int) int64 { return int64(n) * 2 })
				if len(out) != size {
					b.Fatalf("len = %d, want %d", len(out), size)
				}
			}
		})
	}
}

func BenchmarkToWithFilter(b *testing.B) {
	for _, size := range benchSizes {
		input := benchInts(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				out := coreslices.ToWithFilter(input,
					func(n int) bool { return n%2 == 0 },
					func(n int) int64 { return int64(n) },
				)
				if len(out) != size/2 {
					b.Fatalf("len = %d, want %d", len(out), size/2)
				}
			}
		})
	}
}

// BenchmarkFilterMap measures the lazy iterator pipeline Filter then Map,
// drained by a range loop.
func BenchmarkFilterMap(b *testing.B) {
	for _, size := range benchSizes {
		input := benchInts(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sum := 0
				even := coreslices.FilterSeq(coreslices.Map(input, func(n int) int { return n * 3 }),
					func(n int) bool { return n%2 == 0 })
				for v := range even {
					sum += v
				}
				if size > 0 && sum == 0 {
					b.Fatal("empty pipeline")
				}
			}
		})
	}
}

func BenchmarkGroupBy(b *testing.B) {
	for _, size := range benchSizes {
		input := benchInts(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				groups := coreslices.GroupBy(input, func(n int) int { return n % 16 })
				if len(groups) != 16 {
					b.Fatalf("groups = %d, want 16", len(groups))
				}
			}
		})
	}
}

func BenchmarkDeduplicateBy(b *testing.B) {
	type record struct {
		id   string
		data int
	}
	for _, size := range benchSizes {
		input := make([]record, size)
		for i := range size {
			input[i] = record{id: fmt.Sprintf("id-%d", i%(size/2)), data: i}
		}
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				out := coreslices.DeduplicateBy(input, func(r record) string { return r.id })
				if len(out) != size/2 {
					b.Fatalf("len = %d, want %d", len(out), size/2)
				}
			}
		})
	}
}

func BenchmarkReduce(b *testing.B) {
	for _, size := range benchSizes {
		input := benchInts(size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				sum := coreslices.Reduce(input, 0, func(acc, _ int, v int) int { return acc + v })
				if sum != size*(size-1)/2 {
					b.Fatalf("sum = %d", sum)
				}
			}
		})
	}
}

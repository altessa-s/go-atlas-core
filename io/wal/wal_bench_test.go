// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package wal

import (
	"path/filepath"
	"testing"
	"time"
)

// newBenchWAL opens a WAL under b.TempDir with a long fsync interval so
// the background goroutine doesn't contend with the benchmark loop.
// Caller is responsible for Close via b.Cleanup.
func newBenchWAL(b *testing.B, opts ...Option) *WAL {
	b.Helper()
	base := []Option{
		WithMaxSegmentBytes(DefaultMaxSegmentBytes),
		WithFsyncInterval(time.Hour),
	}
	w, _, err := Open(filepath.Join(b.TempDir(), "wal"), append(base, opts...)...)
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	b.Cleanup(func() { _ = w.Close() })
	return w
}

func BenchmarkWAL_Append_Small(b *testing.B) {
	w := newBenchWAL(b)
	payload := make([]byte, 64)

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		if _, err := w.Append(payload); err != nil {
			b.Fatalf("Append: %v", err)
		}
	}
}

func BenchmarkWAL_Append_Large(b *testing.B) {
	w := newBenchWAL(b)
	payload := make([]byte, 4<<10) // 4 KiB

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		if _, err := w.Append(payload); err != nil {
			b.Fatalf("Append: %v", err)
		}
	}
}

// BenchmarkWAL_AppendWithSegmentRoll exercises the segment roll-over path by
// capping the segment size so every few appends trigger a seal + new active
// segment. This measures the amortized cost of the roll syscalls (Sync,
// Close, OpenFile).
func BenchmarkWAL_AppendWithSegmentRoll(b *testing.B) {
	const payloadSize = 256
	// Three records per segment forces frequent rolls.
	segBytes := int64(recordHeaderSize+payloadSize) * 3
	w := newBenchWAL(b, WithMaxSegmentBytes(segBytes))
	payload := make([]byte, payloadSize)

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		if _, err := w.Append(payload); err != nil {
			b.Fatalf("Append: %v", err)
		}
	}
}

func BenchmarkWAL_Ack(b *testing.B) {
	w := newBenchWAL(b)
	payload := make([]byte, 64)

	// Pre-populate enough records so the Ack loop never runs out. The
	// setup cost is excluded via b.ResetTimer below.
	offsets := make([]Offset, 0, 1<<16)
	for range cap(offsets) {
		off, err := w.Append(payload)
		if err != nil {
			b.Fatalf("Append setup: %v", err)
		}
		offsets = append(offsets, off)
	}

	b.ReportAllocs()
	b.ResetTimer()

	var i int
	for b.Loop() {
		w.Ack(offsets[i%len(offsets)])
		i++
	}
}

// BenchmarkWAL_OpenRecover measures the cost of replaying a pre-populated
// WAL directory. Each iteration opens a fresh WAL over the same seeded
// segment files (tb.TempDir for the seed, reused dir for the Open loop),
// so the benchmark reflects the readSegment + CRC verification path.
func BenchmarkWAL_OpenRecover(b *testing.B) {
	const records = 1024
	const payloadSize = 128

	dir := filepath.Join(b.TempDir(), "wal")
	w, _, err := Open(dir,
		WithMaxSegmentBytes(DefaultMaxSegmentBytes),
		WithFsyncInterval(time.Hour),
	)
	if err != nil {
		b.Fatalf("seed Open: %v", err)
	}
	payload := make([]byte, payloadSize)
	for range records {
		if _, err := w.Append(payload); err != nil {
			b.Fatalf("seed Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		b.Fatalf("seed Close: %v", err)
	}
	// Close leaves the unacked segment(s) on disk, which Open will replay.

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w2, recovered, err := Open(dir, WithFsyncInterval(time.Hour))
		if err != nil {
			b.Fatalf("Open: %v", err)
		}
		if len(recovered) != records {
			b.Fatalf("recovered %d, want %d", len(recovered), records)
		}
		if err := w2.Close(); err != nil {
			b.Fatalf("Close: %v", err)
		}
	}
}

// BenchmarkWAL_Append_Parallel measures concurrent producers appending small
// records, the shape a durable dispatcher sees under load.
func BenchmarkWAL_Append_Parallel(b *testing.B) {
	w := newBenchWAL(b)
	payload := make([]byte, 64)

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := w.Append(payload); err != nil {
				b.Errorf("Append: %v", err)
				return
			}
		}
	})
}

// BenchmarkWAL_AppendSync measures a durable write: one record followed by
// an explicit fsync of the active segment.
func BenchmarkWAL_AppendSync(b *testing.B) {
	w := newBenchWAL(b)
	payload := make([]byte, 256)

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		if _, err := w.Append(payload); err != nil {
			b.Fatalf("Append: %v", err)
		}
		if err := w.Sync(); err != nil {
			b.Fatalf("Sync: %v", err)
		}
	}
}

// BenchmarkWAL_AppendAckCycle measures the steady state of a consumer that
// acknowledges every record right after appending it, with small segments
// so fully acknowledged sealed segments keep getting deleted.
func BenchmarkWAL_AppendAckCycle(b *testing.B) {
	const payloadSize = 128
	segBytes := int64(recordHeaderSize+payloadSize) * 64
	w := newBenchWAL(b, WithMaxSegmentBytes(segBytes))
	payload := make([]byte, payloadSize)

	b.ReportAllocs()
	b.SetBytes(payloadSize)
	for b.Loop() {
		off, err := w.Append(payload)
		if err != nil {
			b.Fatalf("Append: %v", err)
		}
		w.Ack(off)
	}
	if st := w.Stats(); st.Pending != 0 {
		b.Fatalf("pending = %d, want 0", st.Pending)
	}
}

// BenchmarkWAL_Stats measures the metrics snapshot polled by exporters.
func BenchmarkWAL_Stats(b *testing.B) {
	w := newBenchWAL(b)
	payload := make([]byte, 64)
	for range 1024 {
		if _, err := w.Append(payload); err != nil {
			b.Fatalf("Append setup: %v", err)
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		if w.Stats().Pending == 0 {
			b.Fatal("no pending records")
		}
	}
}

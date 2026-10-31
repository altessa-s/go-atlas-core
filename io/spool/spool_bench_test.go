// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package spool_test

import (
	"bytes"
	"crypto/sha256"
	"io"
	"strings"
	"testing"

	"github.com/altessa-s/go-atlas-core/io/spool"
)

func BenchmarkNew_InMemory(b *testing.B) {
	payload := []byte(strings.Repeat("x", 1024))
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()

	for b.Loop() {
		sp, err := spool.New(bytes.NewReader(payload))
		if err != nil {
			b.Fatal(err)
		}
		sp.Close()
	}
}

// BenchmarkNew_Spill measures the spill path where content exceeds the memory
// threshold and is written to a temp file.
func BenchmarkNew_Spill(b *testing.B) {
	payload := []byte(strings.Repeat("x", 8<<20)) // 8 MiB
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()

	for b.Loop() {
		sp, err := spool.New(bytes.NewReader(payload), spool.WithMemThreshold(4<<20))
		if err != nil {
			b.Fatal(err)
		}
		sp.Close()
	}
}

// BenchmarkReread measures the reason a spool exists: rewinding and reading
// the materialized content again, from memory and from a spilled temp file.
func BenchmarkReread(b *testing.B) {
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "InMemory_64KiB", size: 64 << 10},
		{name: "Spilled_1MiB", size: 1 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			payload := []byte(strings.Repeat("x", tc.size))
			sp, err := spool.New(bytes.NewReader(payload),
				spool.WithMemThreshold(256<<10),
				spool.WithTempDir(b.TempDir()),
			)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { _ = sp.Close() })
			buf := make([]byte, 32<<10)

			b.SetBytes(int64(tc.size))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := sp.Seek(0, io.SeekStart); err != nil {
					b.Fatal(err)
				}
				n, err := io.CopyBuffer(io.Discard, sp, buf)
				if err != nil {
					b.Fatal(err)
				}
				if n != int64(tc.size) {
					b.Fatalf("read %d bytes, want %d", n, tc.size)
				}
			}
		})
	}
}

// BenchmarkNew_Tee measures materializing content while teeing it to a
// second writer, the shape used to hash an upload as it is spooled.
func BenchmarkNew_Tee(b *testing.B) {
	payload := []byte(strings.Repeat("x", 256<<10))
	tee := sha256.New()
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()

	for b.Loop() {
		tee.Reset()
		sp, err := spool.New(bytes.NewReader(payload), spool.WithTee(tee))
		if err != nil {
			b.Fatal(err)
		}
		_ = sp.Close()
	}
}

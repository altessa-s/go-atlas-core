// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package files_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/altessa-s/go-atlas-core/io/files"
)

// writeBenchFile writes size bytes to name under dir and returns its path.
func writeBenchFile(b *testing.B, dir, name string, size int) string {
	b.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		b.Fatal(err)
	}
	return path
}

// BenchmarkReadRegular measures the checked read of a config- and
// bundle-sized regular file.
func BenchmarkReadRegular(b *testing.B) {
	dir := b.TempDir()
	for _, size := range []int{4 << 10, 1 << 20} {
		path := writeBenchFile(b, dir, fmt.Sprintf("data_%d.bin", size), size)
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				data, err := files.ReadRegular(path)
				if err != nil {
					b.Fatal(err)
				}
				if len(data) != size {
					b.Fatalf("read %d bytes, want %d", len(data), size)
				}
			}
		})
	}
}

// BenchmarkOpenRegularIn measures opening a regular file confined to an
// os.Root, the path untrusted names take.
func BenchmarkOpenRegularIn(b *testing.B) {
	dir := b.TempDir()
	writeBenchFile(b, dir, "payload.json", 128)
	root, err := files.OpenRootDir(dir)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = root.Close() })

	b.ReportAllocs()
	for b.Loop() {
		f, err := files.OpenRegularIn(root, "payload.json")
		if err != nil {
			b.Fatal(err)
		}
		_ = f.Close()
	}
}

// BenchmarkExists measures the stat-based existence checks for a hit and a
// miss.
func BenchmarkExists(b *testing.B) {
	dir := b.TempDir()
	file := writeBenchFile(b, dir, "present.txt", 1)
	missing := filepath.Join(dir, "absent.txt")

	b.Run("FileExists/Hit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if !files.FileExists(file) {
				b.Fatal("file not found")
			}
		}
	})
	b.Run("FileExists/Miss", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if files.FileExists(missing) {
				b.Fatal("missing file reported")
			}
		}
	})
	b.Run("DirExists/Hit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if !files.DirExists(dir) {
				b.Fatal("dir not found")
			}
		}
	})
}

// BenchmarkFindFile measures resolving a relative name against a list of
// search paths where only the last one holds the file.
func BenchmarkFindFile(b *testing.B) {
	bases := make([]string, 4)
	for i := range bases {
		bases[i] = b.TempDir()
	}
	writeBenchFile(b, bases[len(bases)-1], "config.yaml", 16)

	b.ReportAllocs()
	for b.Loop() {
		if files.FindFile("config.yaml", bases) == "" {
			b.Fatal("file not found")
		}
	}
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package hash_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/altessa-s/go-atlas-core/encoding/hash"
)

func BenchmarkSHA256HexString(b *testing.B) {
	s := strings.Repeat("a", 1024)

	b.Run("Core", func(b *testing.B) {
		for b.Loop() {
			hash.SHA256HexString(s)
		}
	})

	b.Run("StdCopy", func(b *testing.B) {
		for b.Loop() {
			sum := sha256.Sum256([]byte(s))
			_ = hex.EncodeToString(sum[:])
		}
	})
}

// BenchmarkSHA256HexBytes measures digesting byte payloads from a short key
// up to a request body.
func BenchmarkSHA256HexBytes(b *testing.B) {
	for _, size := range []int{32, 1 << 10, 64 << 10} {
		data := []byte(strings.Repeat("a", size))
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				if len(hash.SHA256HexBytes(data)) != 64 {
					b.Fatal("unexpected digest length")
				}
			}
		})
	}
}

// BenchmarkHexSum measures hex-encoding the digest of a caller-owned hash.
func BenchmarkHexSum(b *testing.B) {
	h := sha256.New()
	_, _ = h.Write([]byte(strings.Repeat("a", 1024)))
	b.ReportAllocs()
	for b.Loop() {
		if len(hash.HexSum(h)) != 64 {
			b.Fatal("unexpected digest length")
		}
	}
}

func BenchmarkSHA256HexWithPrefix(b *testing.B) {
	s := strings.Repeat("a", 1024)
	for b.Loop() {
		hash.SHA256HexWithPrefix("cache:v1:", s)
	}
}

func BenchmarkSHA256HexWithSalt(b *testing.B) {
	data := []byte(strings.Repeat("a", 1024))
	salt := []byte("0123456789abcdef")

	b.Run("Bytes", func(b *testing.B) {
		for b.Loop() {
			hash.SHA256HexWithSalt(data, salt)
		}
	})

	b.Run("String", func(b *testing.B) {
		s, saltStr := string(data), string(salt)
		for b.Loop() {
			hash.SHA256HexStringWithSalt(s, saltStr)
		}
	})
}

func BenchmarkSHA256Hasher(b *testing.B) {
	data := []byte(strings.Repeat("a", 1024))

	b.Run("Streaming", func(b *testing.B) {
		for b.Loop() {
			h := hash.NewSHA256Hasher()
			h.Write(data)
			_ = h.SumHex()
		}
	})

	b.Run("HexSum", func(b *testing.B) {
		for b.Loop() {
			h := sha256.New()
			h.Write(data)
			_ = hash.HexSum(h)
		}
	})
}

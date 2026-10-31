// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package etag_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/altessa-s/go-atlas-core/encoding/etag"
)

func BenchmarkHash(b *testing.B) {
	g := etag.NewGenerator()
	small := []byte("hello world")
	large := bytes.Repeat([]byte("a"), 64<<10)

	b.Run("small", func(b *testing.B) {
		for b.Loop() {
			_ = g.Hash(small)
		}
	})
	b.Run("large", func(b *testing.B) {
		for b.Loop() {
			_ = g.Hash(large)
		}
	})
}

func BenchmarkHashString(b *testing.B) {
	g := etag.NewGenerator()
	s := strings.Repeat("a", 1024)
	for b.Loop() {
		_ = g.HashString(s)
	}
}

func BenchmarkHashReader(b *testing.B) {
	g := etag.NewGenerator()
	data := bytes.Repeat([]byte("a"), 64<<10)
	r := bytes.NewReader(data)
	for b.Loop() {
		r.Reset(data)
		_, _ = g.HashReader(r)
	}
}

func BenchmarkFromModTime(b *testing.B) {
	mod := time.Unix(0, 0x1234abcd)
	for b.Loop() {
		_ = etag.FromModTime(4096, mod)
	}
}

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		_, _ = etag.Parse(`W/"686d6f3e2c"`)
	}
}

func BenchmarkParseList(b *testing.B) {
	for b.Loop() {
		_, _, _ = etag.ParseList(`"686d6f3e2c", W/"1234567", "abc"`)
	}
}

func BenchmarkVersion(b *testing.B) {
	tag := etag.FromVersion(1234567)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = tag.Version()
	}
}

func BenchmarkParseVersion(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = etag.ParseVersion(`W/"1234567"`)
	}
}

// BenchmarkCheckVersion measures the optimistic-concurrency check an update
// handler runs on its If-Match value.
func BenchmarkCheckVersion(b *testing.B) {
	header := etag.FromVersion(1234567).String()
	b.ReportAllocs()
	for b.Loop() {
		if err := etag.CheckVersion(header, 1234567); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMatch measures the conditional-request comparisons against a
// parsed If-None-Match list.
func BenchmarkMatch(b *testing.B) {
	current := etag.Strong("686d6f3e2c")
	tags, _, err := etag.ParseList(`"aaaa", W/"bbbb", "cccc", W/"686d6f3e2c"`)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Weak", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			matched := false
			for _, t := range tags {
				matched = matched || t.WeakMatch(current)
			}
			if !matched {
				b.Fatal("weak match not found")
			}
		}
	})
	b.Run("Strong", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for _, t := range tags {
				if t.StrongMatch(current) {
					b.Fatal("weak tag matched strongly")
				}
			}
		}
	})
}

// BenchmarkTagString measures rendering a tag for the ETag response header.
func BenchmarkTagString(b *testing.B) {
	for _, tc := range []struct {
		name string
		tag  etag.Tag
	}{
		{name: "Strong", tag: etag.Strong("686d6f3e2c")},
		{name: "Weak", tag: etag.Weak("686d6f3e2c")},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if tc.tag.String() == "" {
					b.Fatal("empty header value")
				}
			}
		})
	}
}

// BenchmarkRandom measures minting a random strong tag.
func BenchmarkRandom(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if etag.Random().IsZero() {
			b.Fatal("zero tag")
		}
	}
}

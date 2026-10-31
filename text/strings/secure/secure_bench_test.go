// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package secure_test

import (
	"strings"
	"testing"

	"github.com/altessa-s/go-atlas-core/text/strings/secure"
)

// BenchmarkNewString_Clear measures the pooled construct/clear cycle for an
// inline-stored secret.
func BenchmarkNewString_Clear(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		ss := secure.NewString("correct horse battery staple")
		ss.Clear()
	}
}

// BenchmarkString_Equal measures the constant-time comparison of two
// pooled secrets.
func BenchmarkString_Equal(b *testing.B) {
	x := secure.NewString("correct horse battery staple")
	y := secure.NewString("correct horse battery staple")
	b.Cleanup(x.Clear)
	b.Cleanup(y.Clear)

	b.ReportAllocs()
	for b.Loop() {
		_ = x.Equal(y)
	}
}

// BenchmarkEqual measures constant-time comparison of plain strings.
func BenchmarkEqual(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = secure.Equal("correct horse battery staple", "correct horse battery stapl3")
	}
}

// BenchmarkPrefixMatch measures the constant-time authorization-scheme check
// run on every authenticated request.
func BenchmarkPrefixMatch(b *testing.B) {
	const header = "Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature"
	b.ReportAllocs()
	for b.Loop() {
		if !secure.PrefixMatch(header, "bearer ") {
			b.Fatal("scheme not matched")
		}
	}
}

// BenchmarkSubstringMatch measures the quadratic constant-time search at
// header-sized and URL-sized inputs.
func BenchmarkSubstringMatch(b *testing.B) {
	for _, tc := range []struct{ name, s string }{
		{name: "Header_64B", s: "Basic dXNlcjpwYXNzd29yZA== realm=\"internal\" token=SeCrEt-value"},
		{name: "URL_256B", s: "https://example.test/api/v1/resources?" + strings.Repeat("filter=a&", 24) + "token=SECRET-value"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if !secure.SubstringMatch(tc.s, "secret-value") {
					b.Fatal("substring not matched")
				}
			}
		})
	}
}

// BenchmarkZeroString measures in-place zeroing of a heap string.
func BenchmarkZeroString(b *testing.B) {
	buf := make([]byte, 64)
	s := string(buf)

	b.ReportAllocs()
	for b.Loop() {
		secure.ZeroString(s)
	}
}

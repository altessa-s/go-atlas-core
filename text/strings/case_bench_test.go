// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package strings_test

import (
	"strings"
	"testing"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

// caseBenchInputs are header- and identifier-sized inputs plus a long one,
// in ASCII and with multi-byte runes, so the ASCII fast path and the rune
// fallback are both measured.
var caseBenchInputs = []struct{ name, lower, upper string }{
	{name: "ASCII_Short", lower: "content-type", upper: "CONTENT-TYPE"},
	{name: "ASCII_Long", lower: strings.Repeat("abcdefgh", 64), upper: strings.Repeat("ABCDEFGH", 64)},
	{name: "Unicode", lower: "größe-über-müller", upper: "GRÖSSE-ÜBER-MÜLLER"},
}

func BenchmarkIsLowercase(b *testing.B) {
	for _, tc := range caseBenchInputs {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if !corestrings.IsLowercase(tc.lower) {
					b.Fatal("lowercase input rejected")
				}
			}
		})
	}
}

func BenchmarkIsUppercase(b *testing.B) {
	for _, tc := range caseBenchInputs {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if !corestrings.IsUppercase(tc.upper) {
					b.Fatal("uppercase input rejected")
				}
			}
		})
	}
}

func BenchmarkIsTrimmed(b *testing.B) {
	for _, tc := range []struct {
		name string
		in   string
		want bool
	}{
		{name: "Trimmed", in: "a value without padding", want: true},
		{name: "Padded", in: "  a padded value\t", want: false},
		{name: "UnicodeSpace", in: " value", want: false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if corestrings.IsTrimmed(tc.in) != tc.want {
					b.Fatalf("IsTrimmed(%q) != %v", tc.in, tc.want)
				}
			}
		})
	}
}

func BenchmarkIsEmptyOrWhitespace(b *testing.B) {
	for _, tc := range []struct {
		name string
		in   string
		want bool
	}{
		{name: "Blank", in: " \t\r\n  ", want: true},
		{name: "Text", in: "   value", want: false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if corestrings.IsEmptyOrWhitespace(tc.in) != tc.want {
					b.Fatalf("IsEmptyOrWhitespace(%q) != %v", tc.in, tc.want)
				}
			}
		})
	}
}

func BenchmarkTrimPrefixFast(b *testing.B) {
	const in = "Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature"
	b.ReportAllocs()
	for b.Loop() {
		if corestrings.TrimPrefixFast(in, "Bearer ") == in {
			b.Fatal("prefix not trimmed")
		}
	}
}

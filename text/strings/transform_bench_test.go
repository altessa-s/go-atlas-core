// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package strings_test

import (
	"testing"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

func BenchmarkToCamelCase(b *testing.B) {
	b.Run("ASCII", func(b *testing.B) {
		for b.Loop() {
			corestrings.ToCamelCase("max_retry_count_per_request")
		}
	})
	b.Run("Unicode", func(b *testing.B) {
		for b.Loop() {
			corestrings.ToCamelCase("größe_über_alles_müller")
		}
	})
}

func BenchmarkScreamingSnakeToCamelCase(b *testing.B) {
	b.Run("ASCII", func(b *testing.B) {
		for b.Loop() {
			corestrings.ScreamingSnakeToCamelCase("MAX_RETRY_COUNT_PER_REQUEST")
		}
	})
	b.Run("Unicode", func(b *testing.B) {
		for b.Loop() {
			corestrings.ScreamingSnakeToCamelCase("GRÖSSE_ÜBER_ALLES_MÜLLER")
		}
	})
}

func BenchmarkToSnakeCase(b *testing.B) {
	for _, tc := range []struct{ name, in string }{
		{name: "ASCII", in: "maxRetryCountPerRequest"},
		{name: "Acronym", in: "HTTPServerTLSConfig"},
		{name: "Unicode", in: "größeÜberAllesMüller"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if corestrings.ToSnakeCase(tc.in) == "" {
					b.Fatal("empty result")
				}
			}
		})
	}
}

func BenchmarkToScreamingSnakeCase(b *testing.B) {
	for _, tc := range []struct{ name, in string }{
		{name: "ASCII", in: "maxRetryCountPerRequest"},
		{name: "Unicode", in: "größeÜberAllesMüller"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if corestrings.ToScreamingSnakeCase(tc.in) == "" {
					b.Fatal("empty result")
				}
			}
		})
	}
}

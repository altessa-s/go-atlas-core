// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package strings_test

import (
	"strings"
	"testing"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

var truncateInput = strings.Repeat("Привет, world! ", 64)

func BenchmarkTruncateRunes(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = corestrings.TruncateRunes(truncateInput, 200, "…")
	}
}

func BenchmarkTruncateBytes(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = corestrings.TruncateBytes(truncateInput, 500, "…")
	}
}

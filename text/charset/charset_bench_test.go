// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package charset_test

import (
	"strings"
	"testing"

	"github.com/altessa-s/go-atlas-core/text/charset"

	"golang.org/x/text/encoding/charmap"
)

func benchInput(b *testing.B) (utf8Body, cp1251Body []byte) {
	b.Helper()
	text := strings.Repeat(cyrillic, 1024)
	legacy, err := charmap.Windows1251.NewEncoder().Bytes([]byte(text))
	if err != nil {
		b.Fatal(err)
	}
	return []byte(text), legacy
}

func BenchmarkDecodeValidUTF8(b *testing.B) {
	body, _ := benchInput(b)
	b.SetBytes(int64(len(body)))
	for b.Loop() {
		_, _ = charset.Decode(body, "text/csv", charmap.Windows1251)
	}
}

func BenchmarkDecodeFallback(b *testing.B) {
	_, body := benchInput(b)
	b.SetBytes(int64(len(body)))
	for b.Loop() {
		_, _ = charset.Decode(body, "text/csv", charmap.Windows1251)
	}
}

func BenchmarkDecodeDeclared(b *testing.B) {
	_, body := benchInput(b)
	b.SetBytes(int64(len(body)))
	for b.Loop() {
		_, _ = charset.Decode(body, "text/csv; charset=windows-1251", nil)
	}
}

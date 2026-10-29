// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package redacted_test

import (
	"encoding/json"
	"testing"

	"github.com/altessa-s/go-atlas-core/types/redacted"
)

func BenchmarkMarshalJSON(b *testing.B) {
	s := redacted.RedactedString("super-secret")
	var sink []byte
	for b.Loop() {
		out, _ := s.MarshalJSON()
		sink = out
	}
	_ = sink
}

func BenchmarkUnmarshalJSON(b *testing.B) {
	data := []byte(`"super-secret"`)
	var s redacted.RedactedString
	for b.Loop() {
		_ = json.Unmarshal(data, &s)
	}
}

func BenchmarkMarshalYAML(b *testing.B) {
	s := redacted.RedactedString("super-secret")
	var sink any
	for b.Loop() {
		out, _ := s.MarshalYAML()
		sink = out
	}
	_ = sink
}

func BenchmarkMarshalText(b *testing.B) {
	s := redacted.RedactedString("super-secret")
	var sink []byte
	for b.Loop() {
		out, _ := s.MarshalText()
		sink = out
	}
	_ = sink
}

func BenchmarkMarshalBSONValue(b *testing.B) {
	s := redacted.RedactedString("super-secret")
	var sink []byte
	for b.Loop() {
		_, out, _ := s.MarshalBSONValue()
		sink = out
	}
	_ = sink
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package serializer_test

import (
	"testing"

	"github.com/altessa-s/go-atlas-core/encoding/serializer"
	"github.com/altessa-s/go-atlas-core/internal/testhelpers"

	jsonv1 "encoding/json"
)

func BenchmarkJSON(b *testing.B) {
	s := &serializer.JSON{}
	val := testhelpers.Person{Name: "Antonio", Age: 30}

	b.Run("Serialize", func(b *testing.B) {
		for b.Loop() {
			_, _ = s.Serialize(val)
		}
	})

	b.Run("Deserialize", func(b *testing.B) {
		data, _ := s.Serialize(val)
		var out testhelpers.Person
		b.ResetTimer()
		for b.Loop() {
			_ = s.Deserialize(data, &out)
		}
	})
}

// BenchmarkJSONCodecs compares [serializer.JSON] (codec=v2) with the
// encoding/json (v1) calls it replaced and with v1 semantics restored
// through jsonv1.DefaultOptionsV1 (codec=v2-v1opts). Compare with benchstat
// -col /codec.
func BenchmarkJSONCodecs(b *testing.B) {
	val := testhelpers.Person{Name: "Antonio", Age: 30}
	for _, codec := range []struct {
		name string
		s    serializer.Serializer
	}{
		{name: "v1", s: v1Serializer{}},
		{name: "v2", s: &serializer.JSON{}},
		{name: "v2-v1opts", s: serializer.NewJSON(jsonv1.DefaultOptionsV1())},
	} {
		data, err := codec.s.Serialize(val)
		if err != nil {
			b.Fatal(err)
		}
		b.Run("op=Serialize/codec="+codec.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := codec.s.Serialize(val); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("op=Deserialize/codec="+codec.name, func(b *testing.B) {
			b.ReportAllocs()
			var out testhelpers.Person
			for b.Loop() {
				if err := codec.s.Deserialize(data, &out); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

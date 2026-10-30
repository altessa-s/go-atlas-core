// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package optional_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/optional"

	jsonv2 "encoding/json/v2"
)

// benchDoc is a struct with Optional fields, the shape the methods are hot
// in. legacyDoc is the same shape routed through MarshalJSON/UnmarshalJSON.
type (
	benchDoc struct {
		Name  optional.Optional[string] `json:"name"`
		Age   optional.Optional[int]    `json:"age"`
		Point optional.Optional[point]  `json:"point"`
		Note  optional.Optional[string] `json:"note"`
	}
	legacyDoc struct {
		Name  legacy[string] `json:"name"`
		Age   legacy[int]    `json:"age"`
		Point legacy[point]  `json:"point"`
		Note  legacy[string] `json:"note"`
	}
)

// BenchmarkJSONCodecs compares v1 and v2 and, for each, the MarshalJSONTo /
// UnmarshalJSONFrom path (impl=to) against the MarshalJSON / UnmarshalJSON
// path (impl=legacy). Compare with benchstat -col /impl or -col /codec.
func BenchmarkJSONCodecs(b *testing.B) {
	doc := benchDoc{
		Name:  optional.Some("hello"),
		Age:   optional.Some(42),
		Point: optional.Some(point{X: 1, Y: 2}),
	}
	ldoc := legacyDoc{Name: legacy[string]{doc.Name}, Age: legacy[int]{doc.Age}, Point: legacy[point]{doc.Point}}
	input, err := json.Marshal(doc)
	require.NoError(b, err)

	codecs := []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{name: "v1", marshal: json.Marshal, unmarshal: json.Unmarshal},
		{
			name:      "v2",
			marshal:   func(v any) ([]byte, error) { return jsonv2.Marshal(v) },
			unmarshal: func(in []byte, v any) error { return jsonv2.Unmarshal(in, v) },
		},
	}
	for _, codec := range codecs {
		for _, impl := range []struct {
			name string
			val  any
			out  func() any
		}{
			{name: "legacy", val: &ldoc, out: func() any { return new(legacyDoc) }},
			{name: "to", val: &doc, out: func() any { return new(benchDoc) }},
		} {
			b.Run("op=Marshal/codec="+codec.name+"/impl="+impl.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := codec.marshal(impl.val); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("op=Unmarshal/codec="+codec.name+"/impl="+impl.name, func(b *testing.B) {
				b.ReportAllocs()
				out := impl.out()
				for b.Loop() {
					if err := codec.unmarshal(input, out); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

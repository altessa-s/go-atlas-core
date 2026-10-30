// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package civil_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/time/civil"
)

// benchRecord is a struct with Date fields, the shape the methods are hot
// in. legacyRecord is the same shape routed through
// MarshalText/UnmarshalJSON.
type (
	benchRecord struct {
		Start civil.Date `json:"start"`
		End   civil.Date `json:"end"`
		Due   civil.Date `json:"due"`
	}
	legacyRecord struct {
		Start legacyDate `json:"start"`
		End   legacyDate `json:"end"`
		Due   legacyDate `json:"due"`
	}
)

// BenchmarkJSONCodecs compares v1 and v2 and, for each, the MarshalJSONTo /
// UnmarshalJSONFrom path (impl=to) against the MarshalText / UnmarshalJSON
// path (impl=legacy). Compare with benchstat -col /impl or -col /codec.
func BenchmarkJSONCodecs(b *testing.B) {
	rec := benchRecord{
		Start: civil.MustNew(2026, time.January, 1),
		End:   civil.MustNew(2026, time.December, 31),
	}
	lrec := legacyRecord{Start: legacyDate{rec.Start}, End: legacyDate{rec.End}}
	input, err := json.Marshal(rec)
	require.NoError(b, err)

	for _, codec := range jsonCodecs {
		for _, impl := range []struct {
			name string
			val  any
			out  func() any
		}{
			{name: "legacy", val: &lrec, out: func() any { return new(legacyRecord) }},
			{name: "to", val: &rec, out: func() any { return new(benchRecord) }},
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

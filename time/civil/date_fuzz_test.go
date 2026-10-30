// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package civil_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/time/civil"
)

// FuzzJSONCodecs pins that MarshalJSONTo and UnmarshalJSONFrom, which both
// JSON codecs prefer over the older methods, keep the wire format and
// decoding of the MarshalText/UnmarshalJSON path under v1 and v2, and that
// v1 and v2 agree with each other on every date.
func FuzzJSONCodecs(f *testing.F) {
	f.Add(2026, 10, 9, []byte(`"2026-10-09"`))
	f.Add(0, 0, 0, []byte("null"))
	f.Add(2026, 2, 30, []byte(`{"Year":2026,"Month":2,"Day":30}`))
	f.Add(1, 1, 1, []byte(` "" `))
	f.Add(9999, 12, 31, []byte("\"2026-10-0\xff\""))

	f.Fuzz(func(t *testing.T, year, month, day int, input []byte) {
		d := civil.Date{Year: year, Month: time.Month(month), Day: day}

		var outs []string
		for _, codec := range jsonCodecs {
			got, err := codec.marshal(d)
			ref, refErr := codec.marshal(legacyDate{d})
			require.Equal(t, refErr == nil, err == nil, "%s: %v vs %v", codec.name, err, refErr)
			require.Equal(t, string(ref), string(got), codec.name)
			if err != nil {
				require.ErrorIs(t, err, civil.ErrInvalidDate, codec.name)
				continue
			}
			outs = append(outs, string(got))

			var back civil.Date
			require.NoError(t, codec.unmarshal(got, &back), codec.name)
			require.Equal(t, d, back, codec.name)
		}
		if len(outs) == len(jsonCodecs) {
			require.Equal(t, outs[0], outs[1], "v1 and v2 must agree on the wire")
		}

		for _, codec := range jsonCodecs {
			got := d
			err := codec.unmarshal(input, &got)
			ref := legacyDate{d}
			refErr := codec.unmarshal(input, &ref)
			require.Equal(t, refErr == nil, err == nil, "%s: %v vs %v", codec.name, err, refErr)
			require.Equal(t, errors.Is(refErr, civil.ErrInvalidDate), errors.Is(err, civil.ErrInvalidDate), codec.name)
			require.Equal(t, ref.d, got, codec.name)
		}
	})
}

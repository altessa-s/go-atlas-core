// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package civil_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/time/civil"

	jsonv2 "encoding/json/v2"
)

// legacyDate is a frozen copy of the MarshalText / UnmarshalJSON path Date
// used before the move to encoding/json/v2, built on encoding/json (v1).
// Encoding it takes the code path every codec used before, so it is the
// reference MarshalJSONTo and UnmarshalJSONFrom must match for v1 callers.
type legacyDate struct{ d civil.Date }

func (l legacyDate) MarshalText() ([]byte, error) { return l.d.MarshalText() }

func (l *legacyDate) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, []byte("null")):
		l.d = civil.Date{}
		return nil
	case len(b) > 0 && b[0] == '{':
		var o struct {
			Year  int
			Month time.Month
			Day   int
		}
		if err := json.Unmarshal(b, &o); err != nil {
			return fmt.Errorf("%w: %w", civil.ErrInvalidDate, err)
		}
		v := civil.Date{Year: o.Year, Month: o.Month, Day: o.Day}
		if !v.IsZero() && !v.Valid() {
			return fmt.Errorf("%w: %04d-%02d-%02d", civil.ErrInvalidDate, v.Year, int(v.Month), v.Day)
		}
		l.d = v
		return nil
	default:
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return fmt.Errorf("%w: %w", civil.ErrInvalidDate, err)
		}
		return l.d.UnmarshalText([]byte(s))
	}
}

// jsonCodecs lists the two standard-library JSON front ends.
var jsonCodecs = []struct {
	name      string
	marshal   func(any) ([]byte, error)
	unmarshal func([]byte, any) error
}{
	{name: "v1", marshal: json.Marshal, unmarshal: json.Unmarshal},
	{
		name:      "v2",
		marshal:   func(v any) ([]byte, error) { return jsonv2.Marshal(v) },
		unmarshal: func(b []byte, v any) error { return jsonv2.Unmarshal(b, v) },
	},
}

func TestMarshalJSONTo_Codecs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		in      civil.Date
		want    string
		wantErr bool
	}{
		{name: "zero", in: civil.Date{}, want: `""`},
		{name: "date", in: civil.MustNew(2026, time.October, 9), want: `"2026-10-09"`},
		{name: "min", in: civil.MustNew(1, time.January, 1), want: `"0001-01-01"`},
		{name: "max", in: civil.MustNew(9999, time.December, 31), want: `"9999-12-31"`},
		{name: "invalid", in: civil.Date{Year: 2026, Month: time.February, Day: 30}, wantErr: true},
		{name: "year_zero", in: civil.Date{Month: time.January, Day: 1}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, codec := range jsonCodecs {
				got, err := codec.marshal(tc.in)
				ref, refErr := codec.marshal(legacyDate{tc.in})
				require.Equal(t, refErr == nil, err == nil, "%s: %v vs %v", codec.name, err, refErr)
				require.Equal(t, string(ref), string(got), "%s must match the MarshalText path", codec.name)
				if tc.wantErr {
					require.ErrorIs(t, err, civil.ErrInvalidDate, codec.name)
					continue
				}
				require.NoError(t, err, codec.name)
				require.Equal(t, tc.want, string(got), codec.name)
			}
		})
	}
}

func TestMarshalJSONTo_StructAndMapKey(t *testing.T) {
	t.Parallel()

	type record struct {
		Due civil.Date         `json:"due"`
		Opt civil.Date         `json:"opt,omitzero"`
		By  map[civil.Date]int `json:"by"`
	}
	in := record{
		Due: civil.MustNew(2026, time.October, 9),
		By:  map[civil.Date]int{civil.MustNew(2026, time.March, 1): 1},
	}
	const want = `{"due":"2026-10-09","by":{"2026-03-01":1}}`

	for _, codec := range jsonCodecs {
		got, err := codec.marshal(in)
		require.NoError(t, err, codec.name)
		require.Equal(t, want, string(got), codec.name)

		var out record
		require.NoError(t, codec.unmarshal(got, &out), codec.name)
		require.Equal(t, in, out, codec.name)
	}
}

func TestUnmarshalJSONFrom_Codecs(t *testing.T) {
	t.Parallel()

	seed := civil.MustNew(2000, time.January, 1)
	for _, tc := range []struct {
		name        string
		input       string
		want        civil.Date
		wantInvalid bool
		wantErr     bool
	}{
		{name: "string", input: `"2026-10-09"`, want: civil.MustNew(2026, time.October, 9)},
		{name: "whitespace", input: " \"2026-10-09\"\n", want: civil.MustNew(2026, time.October, 9)},
		{name: "empty_string", input: `""`, want: civil.Date{}},
		{name: "null", input: "null", want: civil.Date{}},
		{name: "legacy_object", input: `{"Year":2026,"Month":10,"Day":9}`, want: civil.MustNew(2026, time.October, 9)},
		{name: "legacy_object_lowercase", input: `{"year":2026,"month":10,"day":9}`, want: civil.MustNew(2026, time.October, 9)},
		{name: "legacy_zero_object", input: `{"Year":0,"Month":0,"Day":0}`, want: civil.Date{}},
		{name: "legacy_invalid_object", input: `{"Year":2026,"Month":2,"Day":30}`, want: seed, wantInvalid: true},
		{name: "legacy_bad_field", input: `{"Year":"x"}`, want: seed, wantInvalid: true},
		{name: "invalid_date", input: `"2026-02-30"`, want: seed, wantInvalid: true},
		{name: "malformed", input: `"10/09/2026"`, want: seed, wantInvalid: true},
		{name: "number", input: "20261009", want: seed, wantInvalid: true},
		{name: "bool", input: "true", want: seed, wantInvalid: true},
		{name: "array", input: `["2026-10-09"]`, want: seed, wantInvalid: true},
		{name: "truncated", input: `"2026-10`, want: seed, wantErr: true},
		{name: "empty", input: "", want: seed, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, codec := range jsonCodecs {
				got := seed
				err := codec.unmarshal([]byte(tc.input), &got)
				ref := legacyDate{seed}
				refErr := codec.unmarshal([]byte(tc.input), &ref)

				require.Equal(t, refErr == nil, err == nil, "%s must match the legacy UnmarshalJSON: %v vs %v", codec.name, err, refErr)
				require.Equal(t, errors.Is(refErr, civil.ErrInvalidDate), errors.Is(err, civil.ErrInvalidDate), "%s: %v vs %v", codec.name, err, refErr)
				require.Equal(t, ref.d, got, "%s must match the legacy UnmarshalJSON", codec.name)
				require.Equal(t, tc.want, got, codec.name)
				switch {
				case tc.wantInvalid:
					require.ErrorIs(t, err, civil.ErrInvalidDate, codec.name)
				case tc.wantErr:
					require.Error(t, err, codec.name)
				default:
					require.NoError(t, err, codec.name)
				}
			}
		})
	}
}

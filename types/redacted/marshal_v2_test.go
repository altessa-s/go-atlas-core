// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package redacted_test

import (
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/redacted"

	jsonv2 "encoding/json/v2"
)

// legacyString and legacyURL are frozen copies of the MarshalJSON /
// UnmarshalJSON methods RedactedString and URL had before the move to
// encoding/json/v2, built on encoding/json (v1). Encoding them takes the
// code path every codec used before, so they are the reference
// MarshalJSONTo and UnmarshalJSONFrom must match byte for byte for v1
// callers.
type (
	legacyString struct{ v redacted.RedactedString }
	legacyURL    struct{ v redacted.URL }
)

func (l legacyString) MarshalJSON() ([]byte, error) { return json.Marshal(expected) }

func (l *legacyString) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("redacted: unmarshal json: %w", err)
	}
	l.v = redacted.RedactedString(v)
	return nil
}

// MarshalJSON relies on String returning the masked URL, as the original
// method's maskURL call did.
func (l legacyURL) MarshalJSON() ([]byte, error) { return json.Marshal(l.v.String()) }

func (l *legacyURL) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("redacted: unmarshal json: %w", err)
	}
	l.v = redacted.URL(v)
	return nil
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

// requireSameMarshal asserts that got and ref encode to the same bytes, or
// fail together, under codec.
func requireSameMarshal(t *testing.T, marshal func(any) ([]byte, error), got, ref any) []byte {
	t.Helper()

	out, err := marshal(got)
	refOut, refErr := marshal(ref)
	require.Equal(t, refErr == nil, err == nil, "error mismatch: %v vs %v", err, refErr)
	require.Equal(t, string(refOut), string(out), "output must match the legacy MarshalJSON")
	return out
}

func TestRedactedString_MarshalJSONTo(t *testing.T) {
	t.Parallel()

	for _, secret := range []string{"", plain, "<script>&", "a\xffb", `"quoted"`} {
		t.Run(secret, func(t *testing.T) {
			t.Parallel()

			s := redacted.RedactedString(secret)
			v1 := requireSameMarshal(t, json.Marshal, s, legacyString{s})
			v2 := requireSameMarshal(t, jsonCodecs[1].marshal, s, legacyString{s})

			// Each codec keeps its own escaping: v1 escapes HTML, v2 does not.
			require.Equal(t, `"\u003credacted\u003e"`, string(v1))
			require.Equal(t, `"<redacted>"`, string(v2))
		})
	}
}

// TestRedactedString_MarshalJSONTo_NeverLeaks sweeps the shapes and options a
// secret can be marshaled with under v2 and asserts that only the
// placeholder reaches the wire.
func TestRedactedString_MarshalJSONTo_NeverLeaks(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Plain  redacted.RedactedString            `json:"plain"`
		Omit   redacted.RedactedString            `json:"omit,omitzero"`
		Ptr    *redacted.RedactedString           `json:"ptr"`
		Any    any                                `json:"any"`
		Slice  []redacted.RedactedString          `json:"slice"`
		Map    map[string]redacted.RedactedString `json:"map"`
		Quoted redacted.RedactedString            `json:"quoted,string"`
	}
	secret := redacted.RedactedString(plain)
	in := cfg{
		Plain:  secret,
		Omit:   secret,
		Ptr:    &secret,
		Any:    secret,
		Slice:  []redacted.RedactedString{secret},
		Map:    map[string]redacted.RedactedString{"k": secret},
		Quoted: secret,
	}

	for _, tc := range []struct {
		name string
		opts []jsonv2.Options
	}{
		{name: "v2_defaults"},
		{name: "v1_defaults", opts: []jsonv2.Options{json.DefaultOptionsV1()}},
		{name: "html_escape", opts: []jsonv2.Options{jsontext.EscapeForHTML(true)}},
		{name: "multiline", opts: []jsonv2.Options{jsontext.Multiline(true)}},
		{name: "deterministic", opts: []jsonv2.Options{jsonv2.Deterministic(true)}},
		{name: "stringify_numbers", opts: []jsonv2.Options{jsonv2.StringifyNumbers(true)}},
		{name: "allow_invalid_utf8", opts: []jsonv2.Options{jsontext.AllowInvalidUTF8(true)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := jsonv2.Marshal(in, tc.opts...)
			require.NoError(t, err)
			require.NotContains(t, string(out), plain)

			var decoded map[string]any
			require.NoError(t, jsonv2.Unmarshal(out, &decoded))
			for _, key := range []string{"plain", "omit", "ptr", "any", "quoted"} {
				require.Equal(t, expected, decoded[key], key)
			}
			require.Equal(t, []any{expected}, decoded["slice"])
			require.Equal(t, map[string]any{"k": expected}, decoded["map"])
		})
	}
}

func TestRedactedString_MarshalJSONTo_OmitZero(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Token redacted.RedactedString `json:"token,omitzero"`
	}
	for _, codec := range jsonCodecs {
		out, err := codec.marshal(cfg{})
		require.NoError(t, err, codec.name)
		require.Equal(t, `{}`, string(out), codec.name)
	}
}

func TestRedactedString_UnmarshalJSONFrom(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "string", input: `"` + plain + `"`, want: plain},
		{name: "escaped", input: `"\u003c\n"`, want: "<\n"},
		{name: "empty", input: `""`, want: ""},
		{name: "null_clears", input: "null", want: ""},
		{name: "number", input: "123", want: "seed", wantErr: true},
		{name: "object", input: `{"not":"a string"}`, want: "seed", wantErr: true},
		{name: "truncated", input: `"abc`, want: "seed", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, codec := range jsonCodecs {
				got := redacted.RedactedString("seed")
				err := codec.unmarshal([]byte(tc.input), &got)
				ref := legacyString{"seed"}
				refErr := codec.unmarshal([]byte(tc.input), &ref)

				require.Equal(t, refErr == nil, err == nil, "%s must match the legacy UnmarshalJSON: %v vs %v", codec.name, err, refErr)
				require.Equal(t, ref.v, got, "%s must match the legacy UnmarshalJSON", codec.name)
				require.Equal(t, tc.wantErr, err != nil, "%s: %v", codec.name, err)
				require.Equal(t, tc.want, got.Expose(), codec.name)
			}
		})
	}
}

func TestRedactedString_UnmarshalJSONFrom_InvalidUTF8(t *testing.T) {
	t.Parallel()

	input := []byte("\"a\xffb\"")

	var got1 redacted.RedactedString
	require.NoError(t, json.Unmarshal(input, &got1))
	require.Equal(t, "a\ufffdb", got1.Expose())

	var got2 redacted.RedactedString
	require.Error(t, jsonv2.Unmarshal(input, &got2), "v2 rejects invalid UTF-8 by default")
}

func TestURL_MarshalJSONTo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "plain", raw: "http://proxy:3128", want: "http://proxy:3128"},
		{name: "credentials", raw: "http://svc:" + urlSecret + "@proxy:3128", want: "http://svc:xxxxx@proxy:3128"},
		{name: "ambiguous", raw: "http://svc:pre@" + urlSecret + "/rest@proxy:3128", want: expected},
		{name: "unparsable", raw: "http://svc:" + urlSecret + "@[::1", want: expected},
		{name: "html", raw: "http://proxy/?a=<b>&c", want: "http://proxy/?a=<b>&c"},
		{name: "invalid_utf8", raw: "http://pro\xffxy:3128/", want: "http://pro\ufffdxy:3128/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u := redacted.URL(tc.raw)
			for _, codec := range jsonCodecs {
				out := requireSameMarshal(t, codec.marshal, u, legacyURL{u})
				require.NotContains(t, string(out), urlSecret, codec.name)

				var decoded string
				require.NoError(t, json.Unmarshal(out, &decoded), codec.name)
				require.Equal(t, tc.want, decoded, codec.name)
			}
		})
	}
}

func TestURL_UnmarshalJSONFrom(t *testing.T) {
	t.Parallel()

	raw := "http://svc:" + urlSecret + "@proxy:3128"
	for _, tc := range []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "string", input: `"` + raw + `"`, want: raw},
		{name: "null_clears", input: "null", want: ""},
		{name: "bool", input: "true", want: "seed", wantErr: true},
		{name: "array", input: `["x"]`, want: "seed", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, codec := range jsonCodecs {
				got := redacted.URL("seed")
				err := codec.unmarshal([]byte(tc.input), &got)
				ref := legacyURL{"seed"}
				refErr := codec.unmarshal([]byte(tc.input), &ref)

				require.Equal(t, refErr == nil, err == nil, "%s must match the legacy UnmarshalJSON: %v vs %v", codec.name, err, refErr)
				require.Equal(t, ref.v, got, "%s must match the legacy UnmarshalJSON", codec.name)
				require.Equal(t, tc.wantErr, err != nil, "%s: %v", codec.name, err)
				require.Equal(t, tc.want, got.Expose(), codec.name)
			}
		})
	}
}

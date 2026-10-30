// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package optional_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/optional"

	jsonv2 "encoding/json/v2"
)

// legacy is a frozen copy of the MarshalJSON/UnmarshalJSON methods Optional
// had before it moved to encoding/json/v2, built on encoding/json (v1).
// Encoding it takes the code path every codec used before, so it is the
// reference MarshalJSONTo and UnmarshalJSONFrom must match for v1 callers.
type legacy[T any] struct{ o optional.Optional[T] }

func (l legacy[T]) MarshalJSON() ([]byte, error) {
	v, ok := l.o.Get()
	if !ok {
		return []byte("null"), nil
	}
	return json.Marshal(v)
}

func (l *legacy[T]) UnmarshalJSON(data []byte) error {
	if string(bytes.Trim(data, " \t\r\n")) == "null" {
		l.o = optional.None[T]()
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	l.o = optional.Some(v)
	return nil
}

// ptrMarshaler has a pointer-receiver MarshalJSON, which v1 calls only for
// addressable values and v2 always calls.
type ptrMarshaler struct{ X int }

func (*ptrMarshaler) MarshalJSON() ([]byte, error) { return []byte(`"ptr"`), nil }

type point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// requireMarshal asserts the v1 and v2 encodings of o, and that each matches
// the output of the frozen v1 reference byte for byte.
func requireMarshal[T any](t *testing.T, o optional.Optional[T], want1, want2 string) {
	t.Helper()

	got1, err := json.Marshal(o)
	require.NoError(t, err)
	ref1, err := json.Marshal(legacy[T]{o})
	require.NoError(t, err)
	require.Equal(t, want1, string(got1), "v1")
	require.Equal(t, string(ref1), string(got1), "v1 must match the legacy MarshalJSON output")

	got2, err := jsonv2.Marshal(o)
	require.NoError(t, err)
	require.Equal(t, want2, string(got2), "v2")
}

func TestMarshalJSONTo_Codecs(t *testing.T) {
	t.Parallel()

	t.Run("none", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.None[int](), "null", "null")
	})
	t.Run("some_zero", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.Some(0), "0", "0")
	})
	t.Run("some_int", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.Some(42), "42", "42")
	})
	t.Run("some_empty_string", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.Some(""), `""`, `""`)
	})
	t.Run("some_html_string", func(t *testing.T) {
		t.Parallel()
		// Each codec keeps its own escaping: v1 escapes HTML, v2 does not.
		requireMarshal(t, optional.Some("<a&b>"), `"\u003ca\u0026b\u003e"`, `"<a&b>"`)
	})
	t.Run("some_struct", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.Some(point{X: 1, Y: 2}), `{"x":1,"y":2}`, `{"x":1,"y":2}`)
	})
	t.Run("some_slice", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.Some([]int{1, 2}), "[1,2]", "[1,2]")
	})
	t.Run("nested_optional", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.Some(optional.None[int]()), "null", "null")
	})
}

// TestMarshalJSONTo_FollowsCallerCodec pins that Some(v) encodes v the way
// the calling codec encodes a bare v, where v1 and v2 differ.
func TestMarshalJSONTo_FollowsCallerCodec(t *testing.T) {
	t.Parallel()

	t.Run("nil_slice", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.Some([]int(nil)), "null", "[]")
	})

	t.Run("pointer_receiver_marshaler", func(t *testing.T) {
		t.Parallel()
		requireMarshal(t, optional.Some(ptrMarshaler{X: 1}), `{"X":1}`, `"ptr"`)
	})

	t.Run("invalid_utf8", func(t *testing.T) {
		t.Parallel()

		o := optional.Some("a\xffb")
		got1, err := json.Marshal(o)
		require.NoError(t, err)
		require.Equal(t, "\"a\ufffdb\"", string(got1))

		_, err = jsonv2.Marshal(o)
		require.Error(t, err, "v2 rejects invalid UTF-8 by default")
	})
}

func TestMarshalJSONTo_OmitZero(t *testing.T) {
	t.Parallel()

	type doc struct {
		A optional.Optional[int] `json:"a,omitzero"`
		B optional.Optional[int] `json:"b"`
	}

	for _, tc := range []struct {
		name string
		in   doc
		want string
	}{
		{name: "none", in: doc{}, want: `{"b":null}`},
		{name: "some_zero", in: doc{A: optional.Some(0), B: optional.Some(0)}, want: `{"a":0,"b":0}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got1, err := json.Marshal(tc.in)
			require.NoError(t, err)
			got2, err := jsonv2.Marshal(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got1))
			require.Equal(t, tc.want, string(got2))
		})
	}
}

func TestUnmarshalJSONFrom_Codecs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		input   string
		want    optional.Optional[int]
		wantErr bool
	}{
		{name: "null", input: "null", want: optional.None[int]()},
		{name: "null_whitespace", input: " \t\r\nnull\n", want: optional.None[int]()},
		{name: "zero", input: "0", want: optional.Some(0)},
		{name: "int", input: " 42 ", want: optional.Some(42)},
		{name: "wrong_type", input: `"42"`, want: optional.Some(7), wantErr: true},
		{name: "truncated", input: "[", want: optional.Some(7), wantErr: true},
		{name: "trailing_token", input: "null 0", want: optional.Some(7), wantErr: true},
		{name: "empty", input: "", want: optional.Some(7), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, codec := range codecs {
				got := optional.Some(7)
				err := codec.unmarshal([]byte(tc.input), &got)
				ref := legacy[int]{optional.Some(7)}
				refErr := codec.unmarshal([]byte(tc.input), &ref)

				require.Equal(t, refErr == nil, err == nil, "%s must match the legacy UnmarshalJSON: %v vs %v", codec.name, err, refErr)
				require.Equal(t, ref.o, got, "%s must match the legacy UnmarshalJSON", codec.name)
				if tc.wantErr {
					require.Error(t, err, codec.name)
					continue
				}
				require.NoError(t, err, codec.name)
				require.Equal(t, tc.want, got, codec.name)
			}
		})
	}
}

func TestUnmarshalJSONFrom_Struct(t *testing.T) {
	t.Parallel()

	type doc struct {
		A optional.Optional[string] `json:"a"`
		B optional.Optional[string] `json:"b"`
		C optional.Optional[string] `json:"c"`
	}

	for _, codec := range codecs {
		in := doc{C: optional.Some("keep")}
		require.NoError(t, codec.unmarshal([]byte(`{"a":"x","b":null}`), &in), codec.name)
		require.Equal(t, doc{A: optional.Some("x"), C: optional.Some("keep")}, in, codec.name)
	}
}

func TestUnmarshalJSONFrom_InvalidUTF8(t *testing.T) {
	t.Parallel()

	input := []byte("\"a\xffb\"")

	var got1 optional.Optional[string]
	require.NoError(t, json.Unmarshal(input, &got1))
	require.Equal(t, optional.Some("a\ufffdb"), got1)

	got2 := optional.Some("keep")
	require.Error(t, jsonv2.Unmarshal(input, &got2), "v2 rejects invalid UTF-8 by default")
	require.Equal(t, optional.Some("keep"), got2)
}

// codecs lists the two standard-library JSON front ends.
var codecs = []struct {
	name      string
	unmarshal func([]byte, any) error
}{
	{name: "v1", unmarshal: json.Unmarshal},
	{name: "v2", unmarshal: func(b []byte, v any) error { return jsonv2.Unmarshal(b, v) }},
}

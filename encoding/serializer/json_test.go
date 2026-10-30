// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package serializer_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/encoding/serializer"
	"github.com/altessa-s/go-atlas-core/internal/testhelpers"

	jsonv1 "encoding/json"
)

// v1Serializer is the encoding/json (v1) behavior JSON had before it moved to
// encoding/json/v2; the tests compare against it to pin the documented
// differences.
type v1Serializer struct{}

func (v1Serializer) Serialize(data any) ([]byte, error)  { return jsonv1.Marshal(data) }
func (v1Serializer) Deserialize(d []byte, out any) error { return jsonv1.Unmarshal(d, out) }

func TestJSON(t *testing.T) {
	s := &serializer.JSON{}

	val := testhelpers.Person{Name: "Antonio", Age: 30}

	bytes, err := s.Serialize(val)
	require.NoError(t, err, "Serialize failed")

	var loaded testhelpers.Person
	require.NoError(t, s.Deserialize(bytes, &loaded), "Deserialize failed")

	require.True(t, reflect.DeepEqual(val, loaded), "Roundtrip failed: got %v, want %v", loaded, val)
}

func TestJSON_RoundTrip(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		s    *serializer.JSON
	}{
		{name: "zero_value", s: &serializer.JSON{}},
		{name: "no_options", s: serializer.NewJSON()},
		{name: "options", s: serializer.NewJSON(json.Deterministic(true), jsontext.AllowDuplicateNames(true))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			val := testhelpers.Person{Name: "Antonio", Age: 30}
			data, err := tc.s.Serialize(val)
			require.NoError(t, err)
			require.JSONEq(t, `{"name":"Antonio","age":30}`, string(data))

			var loaded testhelpers.Person
			require.NoError(t, tc.s.Deserialize(data, &loaded))
			require.Equal(t, val, loaded)
		})
	}
}

func TestJSON_MatchesV1ForPlainData(t *testing.T) {
	t.Parallel()

	v1 := v1Serializer{}
	v2 := &serializer.JSON{}
	val := map[string]any{"name": "Antonio", "tags": []any{"a", 1.5, true, nil}}

	out1, err := v1.Serialize(val)
	require.NoError(t, err)
	out2, err := v2.Serialize(val)
	require.NoError(t, err)
	require.JSONEq(t, string(out1), string(out2))

	var back1, back2 any
	require.NoError(t, v1.Deserialize(out2, &back1))
	require.NoError(t, v2.Deserialize(out1, &back2))
	require.Equal(t, back1, back2)
}

// TestJSON_StricterDefaults pins the v2 defaults the README documents against
// encoding/json (v1), and that options passed to NewJSON relax them.
func TestJSON_StricterDefaults(t *testing.T) {
	t.Parallel()

	v1 := v1Serializer{}
	v2 := &serializer.JSON{}

	t.Run("invalid_utf8_serialize", func(t *testing.T) {
		t.Parallel()

		in := "a\xffb"
		_, err := v1.Serialize(in)
		require.NoError(t, err)
		_, err = v2.Serialize(in)
		require.Error(t, err)
		_, err = serializer.NewJSON(jsontext.AllowInvalidUTF8(true)).Serialize(in)
		require.NoError(t, err)
	})

	t.Run("invalid_utf8_deserialize", func(t *testing.T) {
		t.Parallel()

		in := []byte("\"a\xffb\"")
		var out string
		require.NoError(t, v1.Deserialize(in, &out))
		require.Error(t, v2.Deserialize(in, &out))
		require.NoError(t, serializer.NewJSON(jsontext.AllowInvalidUTF8(true)).Deserialize(in, &out))
	})

	t.Run("duplicate_names", func(t *testing.T) {
		t.Parallel()

		in := []byte(`{"name":"a","name":"b"}`)
		var out testhelpers.Person
		require.NoError(t, v1.Deserialize(in, &out))
		require.Equal(t, "b", out.Name)

		var syntaxErr *jsontext.SyntacticError
		require.ErrorAs(t, v2.Deserialize(in, &out), &syntaxErr)

		out = testhelpers.Person{}
		require.NoError(t, serializer.NewJSON(jsontext.AllowDuplicateNames(true)).Deserialize(in, &out))
		require.Equal(t, "b", out.Name)
	})

	t.Run("case_sensitive_names", func(t *testing.T) {
		t.Parallel()

		in := []byte(`{"NAME":"a"}`)
		var out1, out2 testhelpers.Person
		require.NoError(t, v1.Deserialize(in, &out1))
		require.Equal(t, "a", out1.Name)
		require.NoError(t, v2.Deserialize(in, &out2))
		require.Empty(t, out2.Name)
	})

	t.Run("nil_slice_and_map", func(t *testing.T) {
		t.Parallel()

		in := struct {
			S []int          `json:"s"`
			M map[string]int `json:"m"`
		}{}
		out1, err := v1.Serialize(in)
		require.NoError(t, err)
		require.JSONEq(t, `{"s":null,"m":null}`, string(out1))
		out2, err := v2.Serialize(in)
		require.NoError(t, err)
		require.JSONEq(t, `{"s":[],"m":{}}`, string(out2))
	})

	t.Run("reject_unknown_members", func(t *testing.T) {
		t.Parallel()

		in := []byte(`{"name":"a","extra":1}`)
		var out testhelpers.Person
		require.NoError(t, v2.Deserialize(in, &out))
		require.Error(t, serializer.NewJSON(json.RejectUnknownMembers(true)).Deserialize(in, &out))
	})
}

func TestNewJSON_DefaultOptionsV1(t *testing.T) {
	t.Parallel()

	v1 := v1Serializer{}
	s := serializer.NewJSON(jsonv1.DefaultOptionsV1())
	in := struct {
		S    []int  `json:"s"`
		Name string `json:"name"`
	}{Name: "<a\xff>"}

	want, err := v1.Serialize(in)
	require.NoError(t, err)
	got, err := s.Serialize(in)
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))

	var out testhelpers.Person
	require.NoError(t, s.Deserialize([]byte(`{"NAME":"a","name":"b"}`), &out))
	require.Equal(t, "b", out.Name)
}

func TestNewJSON_LaterOptionsWin(t *testing.T) {
	t.Parallel()

	s := serializer.NewJSON(jsontext.AllowInvalidUTF8(true), jsontext.AllowInvalidUTF8(false))
	_, err := s.Serialize("a\xffb")
	require.Error(t, err)
}

func TestJSON_DeserializeErrors(t *testing.T) {
	t.Parallel()

	s := &serializer.JSON{}
	var out testhelpers.Person
	require.Error(t, s.Deserialize([]byte(`{`), &out))
	require.Error(t, s.Deserialize([]byte(`{"age":"x"}`), &out))
	require.Error(t, s.Deserialize([]byte(`{}`), out), "non-pointer out")
}

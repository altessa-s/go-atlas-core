// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package redacted_test

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/redacted"
)

// --- JSON ------------------------------------------------------------------

func TestRedactedString_MarshalJSON(t *testing.T) {
	t.Parallel()

	s := redacted.RedactedString(plain)
	data, err := json.Marshal(s)
	require.NoError(t, err)

	var decoded string
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, expected, decoded)
	require.NotContains(t, string(data), plain)
}

func TestRedactedString_MarshalJSON_InStruct(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Password redacted.RedactedString `json:"password"`
	}
	data, err := json.Marshal(cfg{Password: plain})
	require.NoError(t, err)

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, expected, decoded["password"])
	require.NotContains(t, string(data), plain)
}

func TestRedactedString_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	var s redacted.RedactedString
	require.NoError(t, json.Unmarshal([]byte(`"`+plain+`"`), &s))
	require.Equal(t, plain, s.Expose())
}

func TestRedactedString_UnmarshalJSON_Invalid(t *testing.T) {
	t.Parallel()

	var s redacted.RedactedString
	err := json.Unmarshal([]byte(`{"not":"a string"}`), &s)
	require.Error(t, err)
}

func TestRedactedString_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Token redacted.RedactedString `json:"token"`
	}

	var c cfg
	require.NoError(t, json.Unmarshal([]byte(`{"token":"`+plain+`"}`), &c))
	require.Equal(t, plain, c.Token.Expose())

	data, err := json.Marshal(c)
	require.NoError(t, err)

	// json.Marshal HTML-escapes < and >; decode to verify the on-wire string.
	var decoded map[string]string
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, expected, decoded["token"])
	require.NotContains(t, string(data), plain)
}

// --- YAML ------------------------------------------------------------------

func TestRedactedString_MarshalYAML(t *testing.T) {
	t.Parallel()

	v, err := redacted.RedactedString(plain).MarshalYAML()
	require.NoError(t, err)
	require.Equal(t, expected, v)
}

func TestRedactedString_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	var s redacted.RedactedString
	unmarshal := func(out any) error {
		p, ok := out.(*string)
		require.True(t, ok)
		*p = plain
		return nil
	}
	require.NoError(t, s.UnmarshalYAML(unmarshal))
	require.Equal(t, plain, s.Expose())
}

func TestRedactedString_UnmarshalYAML_Error(t *testing.T) {
	t.Parallel()

	var s redacted.RedactedString
	wantErr := errors.New("boom")
	err := s.UnmarshalYAML(func(any) error { return wantErr })
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
}

// --- Text ------------------------------------------------------------------

func TestRedactedString_MarshalText(t *testing.T) {
	t.Parallel()

	data, err := redacted.RedactedString(plain).MarshalText()
	require.NoError(t, err)
	require.Equal(t, expected, string(data))
}

func TestRedactedString_UnmarshalText(t *testing.T) {
	t.Parallel()

	var s redacted.RedactedString
	require.NoError(t, s.UnmarshalText([]byte(plain)))
	require.Equal(t, plain, s.Expose())
}

// --- BSON ------------------------------------------------------------------

func TestRedactedString_MarshalBSONValue(t *testing.T) {
	t.Parallel()

	typ, data, err := redacted.RedactedString(plain).MarshalBSONValue()
	require.NoError(t, err)
	require.Equal(t, byte(0x02), typ)

	// BSON string: little-endian uint32 length (including trailing NUL),
	// UTF-8 bytes, trailing NUL.
	require.Equal(t, uint32(len(expected)+1), binary.LittleEndian.Uint32(data))
	require.Equal(t, expected, string(data[4:len(data)-1]))
	require.Equal(t, byte(0), data[len(data)-1])
	require.NotContains(t, string(data), plain)
}

func TestRedactedString_UnmarshalBSONValue(t *testing.T) {
	t.Parallel()

	data := make([]byte, 4+len(plain)+1)
	binary.LittleEndian.PutUint32(data, uint32(len(plain)+1))
	copy(data[4:], plain)

	var s redacted.RedactedString
	require.NoError(t, s.UnmarshalBSONValue(0x02, data))
	require.Equal(t, plain, s.Expose())
}

func TestRedactedString_UnmarshalBSONValue_Null(t *testing.T) {
	t.Parallel()

	s := redacted.RedactedString(plain)
	require.NoError(t, s.UnmarshalBSONValue(0x0a, nil))
	require.Equal(t, "", s.Expose())
}

func TestRedactedString_UnmarshalBSONValue_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  byte
		data []byte
	}{
		{"wrong type", 0x01, []byte{0, 0, 0, 0, 0}},
		{"too short", 0x02, []byte{1, 0, 0, 0}},
		{"bad length prefix", 0x02, []byte{9, 0, 0, 0, 'a', 'b', 0}},
		{"missing NUL terminator", 0x02, []byte{2, 0, 0, 0, 'a', 'b'}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var s redacted.RedactedString
			err := s.UnmarshalBSONValue(tc.typ, tc.data)
			require.Error(t, err)
		})
	}
}

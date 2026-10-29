// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package redacted_test

import (
	"encoding/json"
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

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package optional_test

import (
	"encoding/json"
	"encoding/json/jsontext"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/optional"

	jsonv2 "encoding/json/v2"
)

func TestMarshalJSONNullBuffersAreIndependent(t *testing.T) {
	t.Parallel()

	a, err := jsonv2.Marshal(optional.None[int]())
	require.NoError(t, err)
	b, err := jsonv2.Marshal(optional.None[string]())
	require.NoError(t, err)
	require.Equal(t, "null", string(a))
	require.Equal(t, "null", string(b))
	// Check before mutation, so a regression cannot corrupt shared state
	// while the package's other tests are running.
	require.NotSame(t, &a[0], &b[0])
	a[0] = 'x'
	require.Equal(t, "null", string(b))
	raw, err := json.Marshal(optional.None[int]())
	require.NoError(t, err)
	require.Equal(t, "null", string(raw))
}

func TestUnmarshalJSONWhitespace(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		input string
		valid bool
	}{
		{name: "space", input: " null ", valid: true},
		{name: "all JSON whitespace", input: "\t\r\n null \n\r\t", valid: true},
		{name: "non-JSON whitespace", input: "\u00a0null\u00a0"},
		{name: "form feed", input: "\fnull"},
		{name: "trailing token", input: "null 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			value := optional.Some(42)
			err := jsonv2.Unmarshal([]byte(tc.input), &value)
			if !tc.valid {
				// v2 decodes in a single pass, so a trailing token is reported
				// after the value was stored; only the error is pinned here.
				var syntaxErr *jsontext.SyntacticError
				require.ErrorAs(t, err, &syntaxErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, optional.None[int](), value)
			}

			// encoding/json (v1) callers keep their error type.
			value = optional.Some(42)
			err = json.Unmarshal([]byte(tc.input), &value)
			if !tc.valid {
				var syntaxErr *json.SyntaxError
				require.ErrorAs(t, err, &syntaxErr)
				require.Equal(t, optional.Some(42), value)
				return
			}
			require.NoError(t, err)
			require.Equal(t, optional.None[int](), value)
		})
	}
}

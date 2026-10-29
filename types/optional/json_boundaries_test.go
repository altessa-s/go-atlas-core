// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package optional_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/optional"
)

func TestMarshalJSONNullBuffersAreIndependent(t *testing.T) {
	t.Parallel()

	a, err := optional.None[int]().MarshalJSON()
	require.NoError(t, err)
	b, err := optional.None[string]().MarshalJSON()
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
			err := value.UnmarshalJSON([]byte(tc.input))
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

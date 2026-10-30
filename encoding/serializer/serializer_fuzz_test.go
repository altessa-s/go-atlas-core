// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package serializer

import (
	"testing"

	"github.com/stretchr/testify/require"

	jsonv1 "encoding/json"
)

func FuzzJSONRoundTrip(f *testing.F) {
	f.Add(`{"key":"value"}`)
	f.Add(`{"num":42,"arr":[1,2,3]}`)
	f.Add(`{}`)

	s := &JSON{}
	f.Fuzz(func(t *testing.T, input string) {
		// Try to deserialize
		var data any
		if err := s.Deserialize([]byte(input), &data); err != nil {
			return // invalid JSON, skip
		}

		// Re-serialize
		out, err := s.Serialize(data)
		require.NoError(t, err, "Serialize failed after successful Deserialize")

		// Deserialize again and compare
		var data2 any
		require.NoError(t, s.Deserialize(out, &data2), "second Deserialize failed")
	})
}

// FuzzJSONAgreesWithV1 pins that the stricter v2 defaults of JSON only ever
// reject input relative to encoding/json (v1): whatever JSON decodes, v1
// decodes to the same value, and JSON round-trips its own output.
func FuzzJSONAgreesWithV1(f *testing.F) {
	f.Add(`{"key":"value"}`)
	f.Add(`{"num":42,"arr":[1,2,3.5e10],"n":null}`)
	f.Add(`{"a":1,"a":2}`)
	f.Add("\"a\xffb\"")
	f.Add(`"\ud800"`)

	v2 := &JSON{}
	f.Fuzz(func(t *testing.T, input string) {
		var got2 any
		if err := v2.Deserialize([]byte(input), &got2); err != nil {
			return // rejected by v2; v1 may still accept it
		}

		var got1 any
		require.NoError(t, jsonv1.Unmarshal([]byte(input), &got1), "v1 must accept what v2 accepts")
		require.Equal(t, got1, got2)

		out, err := v2.Serialize(got2)
		require.NoError(t, err, "Serialize failed after successful Deserialize")

		var again any
		require.NoError(t, v2.Deserialize(out, &again))
		require.Equal(t, got2, again)
	})
}

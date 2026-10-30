// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package optional_test

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/optional"

	jsonv2 "encoding/json/v2"
)

// FuzzSomeZeroIsNotNone is the distinction the type exists for: a value that
// happens to be the zero value is still a value.
//
// A *T or a (T, bool) collapses "absent" and "zero" the moment someone writes
// the wrong check, and the resulting bug — a config field explicitly set to 0
// silently taking the default — is invisible at the call site. The property has
// to hold across the encodings too, since that is where the two states are most
// easily flattened into the same bytes.
func FuzzSomeZeroIsNotNone(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(-1)

	f.Fuzz(func(t *testing.T, v int) {
		some := optional.Some(v)
		none := optional.None[int]()

		require.True(t, some.IsSome())
		require.True(t, none.IsNone())
		require.NotEqual(t, some, none, "Some(%v) must not equal None", v)

		// OrDefault is where the two states become observable to a caller.
		require.Equal(t, v, some.OrDefault(v+1))
		require.Equal(t, v+1, none.OrDefault(v+1))
	})
}

// FuzzJSONRoundTrip pins that both states survive the wire.
//
// Some(zero) encoding to the same bytes as None is the failure that turns this
// type back into the *T it replaced — and it would only show up for the zero
// value, which is exactly the case a hand-written table forgets.
func FuzzJSONRoundTrip(f *testing.F) {
	f.Add(0, true)
	f.Add(42, true)
	f.Add(-7, false)

	f.Fuzz(func(t *testing.T, v int, present bool) {
		original := optional.Of(v, present)

		encoded, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded optional.Optional[int]
		require.NoError(t, json.Unmarshal(encoded, &decoded))

		require.Equal(t, original, decoded, "the value changed on its way through JSON: %s", encoded)

		encodedV2, err := jsonv2.Marshal(original)
		require.NoError(t, err)
		require.Equal(t, string(encoded), string(encodedV2), "v1 and v2 must agree on the wire")

		var decodedV2 optional.Optional[int]
		require.NoError(t, jsonv2.Unmarshal(encodedV2, &decodedV2))
		require.Equal(t, original, decodedV2, "the value changed on its way through JSON v2: %s", encodedV2)
	})
}

// FuzzJSONCodecsMatchLegacy pins that MarshalJSONTo and UnmarshalJSONFrom,
// which replaced MarshalJSON and UnmarshalJSON, produce the same bytes and
// values as the frozen legacy implementation under the same codec, so
// encoding/json (v1) callers see no change.
//
// The only sanctioned difference is the one v2 makes for a bare string: it
// rejects invalid UTF-8 that v1 replaces with U+FFFD.
func FuzzJSONCodecsMatchLegacy(f *testing.F) {
	f.Add("hello", true, []byte(`"hello"`))
	f.Add("", false, []byte("null"))
	f.Add("<&>\u2028", true, []byte(` "\u003c" `))
	f.Add("a\xffb", true, []byte("\"a\xffb\""))

	f.Fuzz(func(t *testing.T, v string, present bool, input []byte) {
		o := optional.Of(v, present)

		got1, err1 := json.Marshal(o)
		ref1, refErr1 := json.Marshal(legacy[string]{o})
		require.Equal(t, refErr1 == nil, err1 == nil)
		require.Equal(t, string(ref1), string(got1), "v1 output changed")

		got2, err2 := jsonv2.Marshal(o)
		ref2, refErr2 := jsonv2.Marshal(legacy[string]{o})
		if utf8.ValidString(v) || !present {
			require.Equal(t, refErr2 == nil, err2 == nil)
			require.Equal(t, string(ref2), string(got2), "v2 output changed")
		} else {
			require.Error(t, err2, "v2 must reject invalid UTF-8")
		}

		for _, codec := range codecs {
			got := optional.Some("seed")
			err := codec.unmarshal(input, &got)
			ref := legacy[string]{optional.Some("seed")}
			refErr := codec.unmarshal(input, &ref)
			require.Equal(t, refErr == nil, err == nil, "%s: %v vs %v", codec.name, err, refErr)
			require.Equal(t, ref.o, got, codec.name)
		}
	})
}

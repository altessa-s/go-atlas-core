// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package redacted_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/redacted"
)

// FuzzJSONCodecsMatchLegacy pins the redaction invariant across both JSON
// codecs: MarshalJSONTo, which replaced MarshalJSON, must emit exactly what
// the frozen legacy MarshalJSON did under the same codec — the fixed
// placeholder for RedactedString and the masked form for URL — so no input
// can reach the wire through the new path. UnmarshalJSONFrom must decode
// every input the way the legacy UnmarshalJSON did.
func FuzzJSONCodecsMatchLegacy(f *testing.F) {
	f.Add(plain, []byte(`"`+plain+`"`))
	f.Add("http://svc:"+urlSecret+"@proxy:3128", []byte("null"))
	f.Add("http://svc:pre@"+urlSecret+"/rest@proxy:3128", []byte(`{"a":1}`))
	f.Add("http://pro\xffxy/<b>", []byte("\"a\xffb\""))
	f.Add("", []byte(""))

	f.Fuzz(func(t *testing.T, secret string, input []byte) {
		s := redacted.RedactedString(secret)
		u := redacted.URL(secret)

		for _, codec := range jsonCodecs {
			out := requireSameMarshal(t, codec.marshal, s, legacyString{s})
			require.Contains(t, []string{`"\u003credacted\u003e"`, `"<redacted>"`}, string(out), codec.name)
			requireSameMarshal(t, codec.marshal, u, legacyURL{u})

			gotS := redacted.RedactedString("seed")
			errS := codec.unmarshal(input, &gotS)
			refS := legacyString{"seed"}
			refErrS := codec.unmarshal(input, &refS)
			require.Equal(t, refErrS == nil, errS == nil, "%s: %v vs %v", codec.name, errS, refErrS)
			require.Equal(t, refS.v, gotS, codec.name)

			gotU := redacted.URL("seed")
			errU := codec.unmarshal(input, &gotU)
			refU := legacyURL{"seed"}
			refErrU := codec.unmarshal(input, &refU)
			require.Equal(t, refErrU == nil, errU == nil, "%s: %v vs %v", codec.name, errU, refErrU)
			require.Equal(t, refU.v, gotU, codec.name)
		}
	})
}

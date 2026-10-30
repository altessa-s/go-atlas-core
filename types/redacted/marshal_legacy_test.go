// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package redacted_test

import (
	"encoding/json/jsontext"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/redacted"

	jsonv2 "encoding/json/v2"
)

// TestMarshalJSON_LegacyEncoders pins the json.Marshaler fallback used by
// encoders that ignore MarshalJSONTo: it must never reveal the secret and
// must match what encoding/json/v2 emits through MarshalJSONTo.
func TestMarshalJSON_LegacyEncoders(t *testing.T) {
	t.Parallel()

	const secret = "s3cr3t-password"

	tests := []struct {
		name string
		v    interface {
			MarshalJSON() ([]byte, error)
		}
		wantV2 any
	}{
		{name: "RedactedString", v: redacted.RedactedString(secret), wantV2: redacted.RedactedString(secret)},
		{name: "URL", v: redacted.URL("postgres://admin:" + secret + "@db:5432/app"), wantV2: redacted.URL("postgres://admin:" + secret + "@db:5432/app")},
		{name: "URL invalid UTF-8", v: redacted.URL("http://u:" + secret + "@h/\xff"), wantV2: redacted.URL("http://u:" + secret + "@h/\xff")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.v.MarshalJSON()
			require.NoError(t, err)
			require.NotContains(t, string(got), secret)
			require.True(t, jsontext.Value(got).IsValid(), "output must be valid JSON: %s", got)

			want, err := jsonv2.Marshal(tc.wantV2)
			require.NoError(t, err)
			require.Equal(t, string(want), string(got))
		})
	}
}

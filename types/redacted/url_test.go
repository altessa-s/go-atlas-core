// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package redacted_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/redacted"
)

const urlSecret = "s3cr3t-pa55"

func TestURL_Masking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty", "", ""},
		{"no credentials", "http://proxy:3128", "http://proxy:3128"},
		{"socks5 no credentials", "socks5h://10.0.0.1:1080", "socks5h://10.0.0.1:1080"},
		{"user and password", "http://svc:" + urlSecret + "@proxy:3128", "http://svc:xxxxx@proxy:3128"},
		{"username only", "socks5://svc@proxy:1080", "socks5://svc@proxy:1080"},
		{"malformed", "http://svc:" + urlSecret + "@proxy example:3128/%zz", "<redacted>"},
		{"no scheme with credentials", "svc:" + urlSecret + "@proxy:3128", "<redacted>"},
		{"unencoded slash in password", "http://svc:prefix@" + urlSecret + "/remaining@proxy:3128", "<redacted>"},
		{"second at in path", "http://svc:pw@proxy:3128/a@" + urlSecret, "<redacted>"},
		{"at in query without user info", "http://proxy:3128/?" + urlSecret + "@x", "<redacted>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u := redacted.URL(tc.raw)
			require.Equal(t, tc.raw, u.Expose(), "Expose returns the plain text")
			require.Equal(t, tc.want, u.String())
		})
	}
}

func TestURL_GoString(t *testing.T) {
	t.Parallel()

	u := redacted.URL("http://svc:" + urlSecret + "@proxy:3128")
	require.Equal(t, "URL{http://svc:xxxxx@proxy:3128}", u.GoString())
}

func TestURL_LogValue(t *testing.T) {
	t.Parallel()

	u := redacted.URL("http://svc:" + urlSecret + "@proxy:3128")
	require.Equal(t, slog.StringValue("http://svc:xxxxx@proxy:3128"), u.LogValue())
}

func TestURL_SlogTextHandler(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	u := redacted.URL("http://svc:" + urlSecret + "@proxy:3128")

	logger.Info("proxy", "url", u)

	out := buf.String()
	require.Contains(t, out, "xxxxx")
	require.NotContains(t, out, urlSecret)
}

func TestURL_IsEmpty(t *testing.T) {
	t.Parallel()

	require.True(t, redacted.URL("").IsEmpty())
	require.False(t, redacted.URL("http://proxy:3128").IsEmpty())
}

func TestURL_IsZero(t *testing.T) {
	t.Parallel()

	require.True(t, redacted.URL("").IsZero())
	require.False(t, redacted.URL("http://proxy:3128").IsZero())
}

func TestURL_MarshalJSON(t *testing.T) {
	t.Parallel()

	u := redacted.URL("http://svc:" + urlSecret + "@proxy:3128")
	data, err := json.Marshal(u)
	require.NoError(t, err)

	var decoded string
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, "http://svc:xxxxx@proxy:3128", decoded)
	require.NotContains(t, string(data), urlSecret)
}

func TestURL_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	var u redacted.URL
	require.NoError(t, json.Unmarshal([]byte(`"http://proxy:3128"`), &u))
	require.Equal(t, "http://proxy:3128", u.Expose())
}

func TestURL_UnmarshalJSON_Null(t *testing.T) {
	t.Parallel()

	u := redacted.URL("http://proxy:3128")
	require.NoError(t, json.Unmarshal([]byte(`null`), &u))
	require.Equal(t, "", u.Expose())
}

func TestURL_UnmarshalJSON_Invalid(t *testing.T) {
	t.Parallel()

	var u redacted.URL
	err := json.Unmarshal([]byte(`{"not":"a string"}`), &u)
	require.Error(t, err)
}

func TestURL_MarshalYAML(t *testing.T) {
	t.Parallel()

	u := redacted.URL("http://svc:" + urlSecret + "@proxy:3128")
	v, err := u.MarshalYAML()
	require.NoError(t, err)
	require.Equal(t, "http://svc:xxxxx@proxy:3128", v)
}

func TestURL_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	var u redacted.URL
	unmarshal := func(out any) error {
		p, ok := out.(*string)
		require.True(t, ok)
		*p = "http://proxy:3128"
		return nil
	}
	require.NoError(t, u.UnmarshalYAML(unmarshal))
	require.Equal(t, "http://proxy:3128", u.Expose())
}

func TestURL_UnmarshalYAML_Error(t *testing.T) {
	t.Parallel()

	var u redacted.URL
	wantErr := errors.New("boom")
	err := u.UnmarshalYAML(func(any) error { return wantErr })
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
}

func TestURL_MarshalText(t *testing.T) {
	t.Parallel()

	u := redacted.URL("http://svc:" + urlSecret + "@proxy:3128")
	data, err := u.MarshalText()
	require.NoError(t, err)
	require.Equal(t, "http://svc:xxxxx@proxy:3128", string(data))
}

func TestURL_UnmarshalText(t *testing.T) {
	t.Parallel()

	var u redacted.URL
	require.NoError(t, u.UnmarshalText([]byte("http://proxy:3128")))
	require.Equal(t, "http://proxy:3128", u.Expose())
}

func TestURL_HasAmbiguousUserInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want bool
	}{
		{"", false},
		{"http://proxy:3128", false},
		{"http://svc:pw@proxy:3128", false},
		{"socks5://svc@proxy:1080", false},
		{"http://svc:prefix@secret/remaining@proxy:3128", true},
		{"http://svc:prefix@secret:1/remaining@proxy:3128", true},
		{"svc:pw@proxy:3128", true},
		{"http://proxy:3128/?a@b", true},
		{"http://svc:pw@proxy example:3128/%zz", true},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, redacted.URL(tc.raw).HasAmbiguousUserInfo())
		})
	}
}

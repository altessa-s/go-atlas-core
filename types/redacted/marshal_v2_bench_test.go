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

// benchCfg is a config struct with redacted fields, the shape the methods
// are hot in. legacyCfg is the same shape routed through
// MarshalJSON/UnmarshalJSON.
type (
	benchCfg struct {
		User     string                  `json:"user"`
		Password redacted.RedactedString `json:"password"`
		Token    redacted.RedactedString `json:"token"`
		Proxy    redacted.URL            `json:"proxy"`
	}
	legacyCfg struct {
		User     string       `json:"user"`
		Password legacyString `json:"password"`
		Token    legacyString `json:"token"`
		Proxy    legacyURL    `json:"proxy"`
	}
)

// BenchmarkJSONCodecs compares v1 and v2 and, for each, the MarshalJSONTo /
// UnmarshalJSONFrom path (impl=to) against the MarshalJSON / UnmarshalJSON
// path (impl=legacy). Compare with benchstat -col /impl or -col /codec.
func BenchmarkJSONCodecs(b *testing.B) {
	proxy := redacted.URL("http://svc:" + urlSecret + "@proxy:3128")
	cfg := benchCfg{User: "svc", Password: plain, Token: plain, Proxy: proxy}
	lcfg := legacyCfg{User: "svc", Password: legacyString{plain}, Token: legacyString{plain}, Proxy: legacyURL{proxy}}
	input, err := json.Marshal(map[string]string{"user": "svc", "password": plain, "token": plain, "proxy": string(proxy)})
	require.NoError(b, err)

	for _, codec := range jsonCodecs {
		for _, impl := range []struct {
			name string
			val  any
			out  func() any
		}{
			{name: "legacy", val: &lcfg, out: func() any { return new(legacyCfg) }},
			{name: "to", val: &cfg, out: func() any { return new(benchCfg) }},
		} {
			b.Run("op=Marshal/codec="+codec.name+"/impl="+impl.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := codec.marshal(impl.val); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("op=Unmarshal/codec="+codec.name+"/impl="+impl.name, func(b *testing.B) {
				b.ReportAllocs()
				out := impl.out()
				for b.Loop() {
					if err := codec.unmarshal(input, out); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

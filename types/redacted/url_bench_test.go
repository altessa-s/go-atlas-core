// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package redacted_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/altessa-s/go-atlas-core/types/redacted"
)

func BenchmarkURL_String_WithCredentials(b *testing.B) {
	u := redacted.URL("http://svc:secret@proxy.internal:3128")
	for b.Loop() {
		_ = u.String()
	}
}

func BenchmarkURL_String_Plain(b *testing.B) {
	u := redacted.URL("http://proxy.internal:3128")
	for b.Loop() {
		_ = u.String()
	}
}

// BenchmarkURL_MarshalJSON measures printing a configuration that holds a
// proxy URL with credentials through encoding/json.
func BenchmarkURL_MarshalJSON(b *testing.B) {
	cfg := struct {
		Proxy redacted.URL `json:"proxy"`
	}{Proxy: redacted.URL("http://svc:secret@proxy.internal:3128")}
	b.ReportAllocs()
	for b.Loop() {
		out, err := json.Marshal(cfg)
		if err != nil {
			b.Fatal(err)
		}
		if bytes.Contains(out, []byte("secret")) {
			b.Fatal("password leaked")
		}
	}
}

// BenchmarkURL_LogValue measures the slog rendering used on every log line
// that carries the URL.
func BenchmarkURL_LogValue(b *testing.B) {
	u := redacted.URL("http://svc:secret@proxy.internal:3128")
	b.ReportAllocs()
	for b.Loop() {
		if u.LogValue().String() == "" {
			b.Fatal("empty log value")
		}
	}
}

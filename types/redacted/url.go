// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package redacted

import (
	"encoding/json/jsontext"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"unicode/utf8"
)

// URL is a named string type for URLs that may embed credentials
// (http://user:pass@host:3128), such as proxy URLs in configuration. Every
// standard output path (fmt, log/slog, encoding/json, gopkg.in/yaml.v3,
// encoding.TextMarshaler) renders the URL with the user-info password replaced
// by "xxxxx", as [url.URL.Redacted] does, so the URL stays readable in printed
// configuration without its secret. A value that cannot be parsed, that
// carries an "@" the parser does not read as user info, or that holds more
// than one "@", renders as the "<redacted>" placeholder because its secret
// part cannot be located. The
// plain text is reachable only through [URL.Expose].
//
// Like [RedactedString], URL keeps reflect.Kind == reflect.String, so config
// loaders that assign string scalars via reflection populate it directly.
// Unlike RedactedString it has no BSON hooks: it is a configuration and
// display type, and the Mongo driver stores it as a plain string.
type URL string

// String implements fmt.Stringer, returning the masked URL.
func (u URL) String() string { return maskURL(string(u)) }

// GoString implements fmt.GoStringer for %#v formatting.
func (u URL) GoString() string { return "URL{" + maskURL(string(u)) + "}" }

// LogValue implements slog.LogValuer, returning the masked URL.
func (u URL) LogValue() slog.Value { return slog.StringValue(maskURL(string(u))) }

// Expose returns the underlying plain-text URL, credentials included. Use it
// only to hand the URL to the code that needs it (url.Parse, a transport) —
// never to log or serialize it.
func (u URL) Expose() string { return string(u) }

// IsEmpty reports whether the underlying value is the empty string.
func (u URL) IsEmpty() bool { return u == "" }

// IsZero reports whether the underlying value is the empty string.
func (u URL) IsZero() bool { return u == "" }

// MarshalJSONTo implements [encoding/json/v2.MarshalerTo], emitting the
// masked URL as a JSON string. Each byte of invalid UTF-8 in the masked URL
// is written as U+FFFD under every codec, as encoding/json does, instead of
// failing under encoding/json/v2.
func (u URL) MarshalJSONTo(enc *jsontext.Encoder) error {
	return enc.WriteToken(jsontext.String(replaceInvalidUTF8(maskURL(string(u)))))
}

// MarshalJSON implements the legacy json.Marshaler interface for encoders
// that do not know [encoding/json/v2.MarshalerTo] (goccy/go-json, jsoniter,
// ...). Without it such encoders reflect on the underlying string and emit
// the unmasked URL. encoding/json and encoding/json/v2 prefer [URL.MarshalJSONTo].
func (u URL) MarshalJSON() ([]byte, error) {
	return jsontext.AppendQuote(nil, replaceInvalidUTF8(maskURL(string(u))))
}

// UnmarshalJSONFrom implements [encoding/json/v2.UnmarshalerFrom], storing a
// JSON string verbatim. JSON null clears the value; other JSON types are
// rejected.
func (u *URL) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	v, err := decodeJSONString(dec)
	if err != nil {
		return err
	}
	*u = URL(v)
	return nil
}

// replaceInvalidUTF8 returns s with each byte that is not part of a valid
// UTF-8 sequence replaced by U+FFFD — one replacement per byte, as
// encoding/json writes them, unlike [strings.ToValidUTF8], which collapses
// runs.
func replaceInvalidUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2*utf8.UTFMax)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// MarshalYAML implements yaml.Marshaler, emitting the masked URL scalar.
func (u URL) MarshalYAML() (any, error) {
	return maskURL(string(u)), nil
}

// UnmarshalYAML uses the callback unmarshaler supported by yaml.v3 and stores
// the scalar verbatim.
func (u *URL) UnmarshalYAML(unmarshal func(any) error) error {
	var v string
	if err := unmarshal(&v); err != nil {
		return fmt.Errorf("redacted: unmarshal yaml: %w", err)
	}
	*u = URL(v)
	return nil
}

// MarshalText implements encoding.TextMarshaler, returning the masked URL.
func (u URL) MarshalText() ([]byte, error) {
	return []byte(maskURL(string(u))), nil
}

// UnmarshalText implements encoding.TextUnmarshaler, storing the bytes
// verbatim.
func (u *URL) UnmarshalText(data []byte) error {
	*u = URL(data)
	return nil
}

// HasAmbiguousUserInfo reports whether the URL holds an "@" that is not the
// single separator of a parsed user info: more than one "@" (an unencoded "/"
// or "@" in the password moves part of the credentials into the host or path,
// as in "http://svc:pre@secret/rest@proxy:3128"), an "@" without parsed user
// info ("svc:pass@proxy:3128" without a scheme), or an "@" in a value that
// does not parse. Such a URL renders as the "<redacted>" placeholder, and
// configuration validators should reject it: its host or path may carry part
// of a secret into DNS lookups, dial errors and logs.
func (u URL) HasAmbiguousUserInfo() bool {
	raw := string(u)
	if !strings.Contains(raw, "@") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return true
	}
	return ambiguousUserInfo(raw, parsed)
}

// ambiguousUserInfo is the predicate behind [URL.HasAmbiguousUserInfo] for an
// already parsed raw URL.
func ambiguousUserInfo(raw string, parsed *url.URL) bool {
	ats := strings.Count(raw, "@")
	return ats > 1 || (ats == 1 && parsed.User == nil)
}

// maskURL returns raw with its user-info password replaced by "xxxxx". It
// returns raw unchanged when it carries no "@" at all, and the placeholder
// when it does not parse or its user info is ambiguous (see
// [URL.HasAmbiguousUserInfo]), since Redacted would print the part of the
// secret that landed in the host or path.
func maskURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	switch {
	case err != nil:
		return placeholder
	case !strings.Contains(raw, "@"):
		return raw
	case ambiguousUserInfo(raw, parsed):
		return placeholder
	default:
		return parsed.Redacted()
	}
}

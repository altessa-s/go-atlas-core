// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package network_test

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/errors/network"
)

// FuzzNetworkMatchersLookOnlyInsideAURLError pins the shape these classifiers
// require before they will say yes.
//
// They are string matchers over an error message, which is the fragile kind:
// the message comes from net/http and, in a proxy or a retry wrapper, can carry
// text a remote peer influenced. Both are documented to answer only for a
// *url.Error, so a message that merely mentions redirects — in an unrelated
// error, or in the URL itself — must not be enough. Otherwise a retry loop
// classifies a failure by a substring an attacker put in a hostname.
func FuzzNetworkMatchersLookOnlyInsideAURLError(f *testing.F) {
	f.Add("stopped after 10 redirects", true)
	f.Add("unsupported protocol scheme \"ftp\"", true)
	f.Add("random error string", true)
	f.Add("stopped after 10 redirects", false)
	f.Add("", true)

	f.Fuzz(func(t *testing.T, message string, wrapped bool) {
		plain := errors.New(message)

		if !wrapped {
			// Not a *url.Error: neither matcher may claim it, whatever it says.
			require.False(t, network.IsResourceRedirects(plain),
				"a plain error was classified as a redirect failure: %q", message)
			require.False(t, network.IsUnsupportedProtocolScheme(plain),
				"a plain error was classified as a scheme failure: %q", message)
			return
		}

		// The URL field is caller-controlled — in a proxy it carries a target a
		// client chose — while the verdict is documented to come from the
		// wrapped error's message. Two errors differing only in URL must
		// therefore classify identically; restating the regex here instead
		// would just be guessing at it.
		withURL := &url.Error{Op: "Get", URL: "http://" + message, Err: plain}
		withoutURL := &url.Error{Op: "Get", URL: "http://example.com", Err: plain}

		require.Equal(t, network.IsResourceRedirects(withoutURL), network.IsResourceRedirects(withURL),
			"the redirect verdict changed with the URL field: %q", message)
		require.Equal(t, network.IsUnsupportedProtocolScheme(withoutURL), network.IsUnsupportedProtocolScheme(withURL),
			"the scheme verdict changed with the URL field: %q", message)
	})
}

// FuzzWrappingPreservesClassification pins that the matchers see through the
// repo's own wrappers.
//
// Every call site wraps errors with network.Wrap on the way up, so a
// classifier that only inspected the outermost error would stop recognizing a
// redirect or a canceled context the moment someone added context to the
// message — and the retry policy built on it would silently change behavior.
func FuzzWrappingPreservesClassification(f *testing.F) {
	f.Add("stopped after 10 redirects", "fetching config", uint8(3))
	f.Add("unsupported protocol scheme", "", uint8(0))
	f.Add("plain", "op", uint8(1))

	f.Fuzz(func(t *testing.T, message, context string, depth uint8) {
		inner := &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New(message)}

		wantRedirects := network.IsResourceRedirects(inner)
		wantScheme := network.IsUnsupportedProtocolScheme(inner)

		var err error = inner
		for range int(depth) % 8 {
			err = fmt.Errorf("%s: %w", context, err)
		}

		require.Equal(t, wantRedirects, network.IsResourceRedirects(err),
			"wrapping %d deep changed the redirect verdict for %q", depth%8, message)
		require.Equal(t, wantScheme, network.IsUnsupportedProtocolScheme(err),
			"wrapping %d deep changed the scheme verdict for %q", depth%8, message)
	})
}

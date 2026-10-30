// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package secure_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/text/strings/secure"
)

func TestEqual(t *testing.T) {
	require.True(t, secure.Equal("secret", "secret"), "Equal failed for equal strings")
	require.False(t, secure.Equal("secret", "wrong"), "Equal passed for different strings")
}

func TestPrefixMatch(t *testing.T) {
	require.True(t, secure.PrefixMatch("Bearer token", "bearer"), "PrefixMatch failed")
}

func TestSubstringMatch(t *testing.T) {
	tests := []struct {
		name      string
		s, substr string
		want      bool
	}{
		{"found", "Hello World", "world", true},
		{"not found", "Hello", "xyz", false},
		{"empty substr", "anything", "", true},
		{"shorter string", "Hi", "Hello", false},
		{"exact match", "hello", "hello", true},
		{"case insensitive", "ABC", "abc", true},
		{"at start", "foobar", "foo", true},
		{"at end", "foobar", "bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, secure.SubstringMatch(tt.s, tt.substr))
		})
	}
}

func TestPrefixMatch_ShorterString(t *testing.T) {
	require.False(t, secure.PrefixMatch("Hi", "Hello"), "shorter string should not match longer prefix")
}

func TestPrefixMatch_ExactMatch(t *testing.T) {
	require.True(t, secure.PrefixMatch("bearer", "bearer"), "exact match should succeed")
}

func TestEqual_EdgeCases(t *testing.T) {
	t.Parallel()

	require.True(t, secure.Equal("", ""), "two empty strings should be equal")
	require.False(t, secure.Equal("a", "ab"), "different lengths should not be equal")
}

func TestPrefixMatch_EmptyPrefix(t *testing.T) {
	t.Parallel()

	require.True(t, secure.PrefixMatch("anything", ""), "empty prefix should match anything")
}

func TestSubstringMatch_WholeString(t *testing.T) {
	t.Parallel()

	require.True(t, secure.SubstringMatch("secret", "secret"), "substr equal to s should match")
}

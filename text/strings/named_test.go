// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package strings_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	corestrings "github.com/altessa-s/go-atlas-core/text/strings"
)

type (
	name    string
	namePtr *string
)

func TestIsEmpty_NamedTypes(t *testing.T) {
	t.Parallel()

	hello, blank := "hello", " \t"
	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{"named string", corestrings.IsEmpty(name("hello")), false},
		{"named empty string", corestrings.IsEmpty(name("")), true},
		{"named whitespace string", corestrings.IsEmpty(name(" \t")), true},
		{"named pointer", corestrings.IsEmpty(namePtr(&hello)), false},
		{"named pointer to whitespace", corestrings.IsEmpty(namePtr(&blank)), true},
		{"nil named pointer", corestrings.IsEmpty(namePtr(nil)), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.got)
		})
	}
}

func TestToPtr_NamedTypes(t *testing.T) {
	t.Parallel()

	t.Run("named string", func(t *testing.T) {
		t.Parallel()
		got := corestrings.ToPtr(name("hello"))
		require.NotNil(t, got)
		require.Equal(t, "hello", *got)
	})
	t.Run("named empty and whitespace strings", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, corestrings.ToPtr(name("")))
		require.Nil(t, corestrings.ToPtr(name("  ")))
	})
	t.Run("named pointer returns the original pointer", func(t *testing.T) {
		t.Parallel()
		hello := "hello"
		got := corestrings.ToPtr(namePtr(&hello))
		require.Same(t, &hello, got)
	})
	t.Run("named pointer to whitespace and nil named pointer", func(t *testing.T) {
		t.Parallel()
		blank := " "
		require.Nil(t, corestrings.ToPtr(namePtr(&blank)))
		require.Nil(t, corestrings.ToPtr(namePtr(nil)))
	})
}

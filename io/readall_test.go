// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package io_test

import (
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	coreio "github.com/altessa-s/go-atlas-core/io"
)

// countingReader records how many bytes were read from it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func TestReadAllLimited(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		limit    int64
		want     string
		exceeded bool
	}{
		{name: "empty", input: "", limit: 4, want: ""},
		{name: "below limit", input: "abc", limit: 4, want: "abc"},
		{name: "at limit", input: "abcd", limit: 4, want: "abcd"},
		{name: "one over", input: "abcde", limit: 4, want: "abcd", exceeded: true},
		{name: "far over", input: strings.Repeat("x", 100), limit: 4, want: "xxxx", exceeded: true},
		{name: "zero limit empty", input: "", limit: 0, want: ""},
		{name: "zero limit data", input: "a", limit: 0, want: "", exceeded: true},
		{name: "negative limit empty", input: "", limit: -1, want: ""},
		{name: "negative limit data", input: "a", limit: -5, want: "", exceeded: true},
		{name: "max limit", input: "abc", limit: math.MaxInt64, want: "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cr := &countingReader{r: strings.NewReader(tc.input)}
			got, err := coreio.ReadAllLimited(cr, tc.limit)
			if tc.exceeded {
				require.ErrorIs(t, err, coreio.ErrReadLimitExceeded)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, string(got))
			if tc.limit >= 0 && tc.limit < math.MaxInt64 {
				require.LessOrEqual(t, cr.n, tc.limit+1, "must read at most n+1 bytes")
			}
		})
	}
}

func TestReadAllLimitedReadError(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	r := io.MultiReader(strings.NewReader("ab"), coreio.NewErrorReader(errBoom))
	got, err := coreio.ReadAllLimited(r, 10)
	require.ErrorIs(t, err, errBoom)
	require.NotErrorIs(t, err, coreio.ErrReadLimitExceeded)
	require.Equal(t, "ab", string(got))
}

// dataAndErrorReader returns all its data together with err in one Read call,
// which io.Reader permits.
type dataAndErrorReader struct {
	data string
	err  error
	done bool
}

func (r *dataAndErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

func TestReadAllLimitedOverflowWithReadError(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	got, err := coreio.ReadAllLimited(&dataAndErrorReader{data: "abcde", err: errBoom}, 4)
	require.ErrorIs(t, err, coreio.ErrReadLimitExceeded)
	require.ErrorIs(t, err, errBoom)
	require.Equal(t, "abcd", string(got))
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package io_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	coreio "github.com/altessa-s/go-atlas-core/io"
)

var errCloseFailed = errors.New("close failed")

type failingCloser struct{ io.Reader }

func (failingCloser) Close() error { return errCloseFailed }

func TestLimitedReadCloser_Close(t *testing.T) {
	t.Parallel()

	t.Run("nil closer returns nil", func(t *testing.T) {
		t.Parallel()
		lrc := coreio.NewLimitedReadCloser(nil, 10)
		require.NoError(t, lrc.Close())
	})

	t.Run("underlying error is returned", func(t *testing.T) {
		t.Parallel()
		lrc := coreio.NewLimitedReadCloser(failingCloser{strings.NewReader("x")}, 10)
		require.ErrorIs(t, lrc.Close(), errCloseFailed)
	})
}

func TestPutBuffer(t *testing.T) {
	t.Parallel()

	t.Run("nil is a no-op", func(t *testing.T) {
		t.Parallel()
		require.NotPanics(t, func() { coreio.PutBuffer(nil) })
	})

	t.Run("pooled buffer is reset", func(t *testing.T) {
		t.Parallel()
		buf := bytes.NewBufferString("data")
		coreio.PutBuffer(buf)
		require.Zero(t, buf.Len())
	})

	t.Run("oversized buffer is discarded untouched", func(t *testing.T) {
		t.Parallel()
		buf := bytes.NewBuffer(make([]byte, 0, 64*1024+1))
		buf.WriteString("data")
		coreio.PutBuffer(buf)
		require.Equal(t, "data", buf.String(), "a buffer above the threshold must not be reset into the pool")

		got := coreio.GetBuffer()
		require.NotSame(t, buf, got)
		require.Zero(t, got.Len())
		coreio.PutBuffer(got)
	})
}

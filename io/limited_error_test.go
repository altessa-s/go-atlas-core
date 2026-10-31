// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package io_test

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	coreio "github.com/altessa-s/go-atlas-core/io"
)

func TestLimitedReadCloserPreservesReadErrorAtLimit(t *testing.T) {
	t.Parallel()
	failure := errors.New("reader failed")
	reader := coreio.NewLimitedReadCloser(io.NopCloser(&dataAndErrorReader{data: "abcdef", err: failure}), 5)
	data, err := io.ReadAll(reader)
	require.Equal(t, "abcde", string(data))
	require.ErrorIs(t, err, coreio.ErrReadLimitExceeded)
	require.ErrorIs(t, err, failure)
}

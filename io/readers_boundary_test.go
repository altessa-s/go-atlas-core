// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package io_test

import (
	"io"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	coreio "github.com/altessa-s/go-atlas-core/io"
)

func TestLimitedReadCloser_MaxLimit(t *testing.T) {
	t.Parallel()
	r := coreio.NewLimitedReadCloser(io.NopCloser(strings.NewReader("payload")), math.MaxInt64)
	defer r.Close()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "payload", string(data))
}

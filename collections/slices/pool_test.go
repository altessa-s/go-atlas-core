// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package slices_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	coreslices "github.com/altessa-s/go-atlas-core/collections/slices"
)

func TestPoolPutClearsTruncatedElements(t *testing.T) {
	t.Parallel()
	pool := coreslices.NewPool[*int](4)
	buf := pool.Get()
	*buf = append(*buf, new(1), new(2), new(3))
	backing := (*buf)[:cap(*buf)]
	*buf = (*buf)[:1]
	pool.Put(buf)
	// Inspect the released allocation directly: sync.Pool may discard it,
	// so another Get cannot reliably verify that Put removed the references.
	for _, value := range backing {
		require.Nil(t, value)
	}
}

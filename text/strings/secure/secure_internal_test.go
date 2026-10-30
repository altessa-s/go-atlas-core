// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package secure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCleanupSecureString exercises the runtime.AddCleanup callback that
// NewString registers for heap-backed storage. The callback is otherwise
// only reachable asynchronously after the owning [String] becomes
// unreachable and is collected, which cannot be driven deterministically
// from a black-box test without sleeping on the GC — so this is a
// white-box test calling the unexported function directly.
func TestCleanupSecureString(t *testing.T) {
	t.Parallel()

	data := []byte("heap-secret")
	cleanupSecureString(data)

	for i, b := range data {
		require.Equal(t, byte(0), b, "byte[%d] should be zeroed", i)
	}
}

// TestCleanupSecureString_Nil pins that a nil data slice (never actually
// produced by NewString, but defensive per the nil check) is a no-op.
func TestCleanupSecureString_Nil(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() { cleanupSecureString(nil) })
}

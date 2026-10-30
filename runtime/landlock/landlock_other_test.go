// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

//go:build !linux

// Tests in this file exercise the non-Linux stub platform dispatch
// (apply, Supported, ABIVersion in landlock_other.go) plus the
// post-validation path inside Apply (cleanPaths and the platform
// dispatch call) that landlock_test.go deliberately avoids on Linux
// because a real Apply call there is irreversible for the test process.
// On a non-Linux platform Apply with well-formed options is always a
// safe no-op (apply always returns ErrUnsupported), so it can run
// directly here.

package landlock_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/landlock"
)

func TestSupported_False(t *testing.T) {
	t.Parallel()

	require.False(t, landlock.Supported(), "Landlock is a Linux-only LSM")
}

func TestABIVersion_Unsupported(t *testing.T) {
	t.Parallel()

	v, err := landlock.ABIVersion()
	require.Zero(t, v)
	require.ErrorIs(t, err, landlock.ErrUnsupported)
}

// TestApply_WellFormedOptions_ReturnsUnsupported exercises the post-
// validation path of Apply (cleanPaths plus the platform dispatch) with
// well-formed, absolute paths. On this platform the call is a pure,
// side-effect-free no-op: apply unconditionally returns ErrUnsupported
// without touching the filesystem or process state.
func TestApply_WellFormedOptions_ReturnsUnsupported(t *testing.T) {
	t.Parallel()

	err := landlock.Apply(
		landlock.WithReadPaths("/etc/", "/usr/lib/../lib"),
		landlock.WithReadWritePaths("/var/log/myservice/"),
	)
	require.Error(t, err)
	require.True(t, errors.Is(err, landlock.ErrUnsupported))
	require.False(t, errors.Is(err, landlock.ErrInvalidOption))
}

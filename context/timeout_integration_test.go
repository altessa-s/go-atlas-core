//go:build integration

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that let the derived contexts expire on real
// timers. Lower bounds are exact (a context never expires before its
// deadline); upper bounds are generous so a loaded machine cannot fail
// them.

package context_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corecontext "github.com/altessa-s/go-atlas-core/context"
)

// slack bounds how late a real timer may fire.
const slack = 2 * time.Second

// requireExpiresAfter waits for ctx to end and requires it to end with
// DeadlineExceeded no earlier than want after start.
func requireExpiresAfter(t *testing.T, ctx context.Context, start time.Time, want time.Duration) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(want + slack):
		require.FailNow(t, "context did not expire", "want expiry after %v", want)
	}
	elapsed := time.Since(start)
	require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
	require.GreaterOrEqual(t, elapsed, want)
}

func TestTimeouts_ExpireOnRealTimers(t *testing.T) {
	t.Parallel()

	const timeout = 30 * time.Millisecond
	tests := []struct {
		name   string
		derive func(context.Context, time.Duration) (context.Context, context.CancelFunc)
	}{
		{name: "ApplyTimeout", derive: corecontext.ApplyTimeout},
		{name: "WithDefault", derive: corecontext.WithDefault},
		{name: "WithMaxTimeout", derive: corecontext.WithMaxTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			start := time.Now()
			ctx, cancel := tc.derive(t.Context(), timeout)
			defer cancel()
			requireExpiresAfter(t, ctx, start, timeout)
		})
	}
}

func TestWithMaxTimeout_CapsLongerParentDeadline(t *testing.T) {
	t.Parallel()

	const capTimeout = 30 * time.Millisecond
	parent, cancelParent := context.WithTimeout(t.Context(), time.Hour)
	defer cancelParent()

	start := time.Now()
	ctx, cancel := corecontext.WithMaxTimeout(parent, capTimeout)
	defer cancel()
	requireExpiresAfter(t, ctx, start, capTimeout)
	require.NoError(t, parent.Err(), "the cap must not end the parent")
}

func TestWithMaxTimeout_KeepsSoonerParentDeadline(t *testing.T) {
	t.Parallel()

	const parentTimeout = 30 * time.Millisecond
	start := time.Now()
	parent, cancelParent := context.WithTimeout(t.Context(), parentTimeout)
	defer cancelParent()

	ctx, cancel := corecontext.WithMaxTimeout(parent, time.Hour)
	defer cancel()
	requireExpiresAfter(t, ctx, start, parentTimeout)
}

func TestApplyTimeout_ParentDeadlineWins(t *testing.T) {
	t.Parallel()

	// A parent deadline later than the timeout still wins: ApplyTimeout
	// only fills in a missing deadline, it never shortens one.
	const parentTimeout = 60 * time.Millisecond
	start := time.Now()
	parent, cancelParent := context.WithTimeout(t.Context(), parentTimeout)
	defer cancelParent()

	ctx, cancel := corecontext.ApplyTimeout(parent, 10*time.Millisecond)
	defer cancel()
	requireExpiresAfter(t, ctx, start, parentTimeout)
}

func TestTimeouts_CancelBeforeExpiry(t *testing.T) {
	t.Parallel()

	ctx, cancel := corecontext.WithMaxTimeout(t.Context(), time.Hour)
	time.AfterFunc(10*time.Millisecond, cancel)

	select {
	case <-ctx.Done():
	case <-time.After(slack):
		require.FailNow(t, "cancel did not end the context")
	}
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}

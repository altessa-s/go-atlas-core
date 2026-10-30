// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package testhelpers_test

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/internal/testhelpers"
)

// tbRecorder is a testing.TB double that records failures and cleanups
// instead of acting on them, so a test can run VerifyNoLeaks's cleanup on
// demand and inspect what it reported.
type tbRecorder struct {
	testing.TB

	mu       sync.Mutex
	errors   []string
	cleanups []func()
}

func (r *tbRecorder) Helper() {}

func (r *tbRecorder) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *tbRecorder) Cleanup(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanups = append(r.cleanups, f)
}

func (r *tbRecorder) runCleanups() {
	r.mu.Lock()
	cleanups := slices.Clone(r.cleanups)
	r.mu.Unlock()
	for _, f := range slices.Backward(cleanups) {
		f()
	}
}

func (r *tbRecorder) reported() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.errors)
}

// leakBlockedOnPrivateChannel starts a goroutine blocked forever on a channel
// that nothing else references: the textbook leak. Its name appears in the
// leaked stack, which the tests use to recognize it.
func leakBlockedOnPrivateChannel() {
	ch := make(chan struct{})
	go func() { <-ch }()
}

// waitLeaked polls tracker until it reports at least one leaked stack: the
// leaking goroutine may not have parked yet when the profile is first taken.
func waitLeaked(tb testing.TB, tracker *testhelpers.GoroutineTracker) []string {
	tb.Helper()
	var leaked []string
	require.Eventually(tb, func() bool {
		var err error
		leaked, err = tracker.Leaked()
		return err == nil && len(leaked) > 0
	}, 5*time.Second, 10*time.Millisecond, "the leaked goroutine was never reported")
	return leaked
}

func TestGoroutineTracker_ReportsGoroutineBlockedOnUnreachableChannel(t *testing.T) {
	t.Parallel()

	tracker := testhelpers.TrackGoroutines()
	leakBlockedOnPrivateChannel()

	leaked := waitLeaked(t, tracker)
	require.Len(t, leaked, 1)
	require.Contains(t, leaked[0], "leakBlockedOnPrivateChannel")
}

func TestGoroutineTracker_IgnoresOtherTrackersLeaks(t *testing.T) {
	t.Parallel()

	leaky := testhelpers.TrackGoroutines()
	leakBlockedOnPrivateChannel()
	waitLeaked(t, leaky)

	// A tracker created after the leak on the same goroutine relabels it;
	// the earlier leak carries the old label and is not attributed here.
	clean := testhelpers.TrackGoroutines()
	leaked, err := clean.Leaked()
	require.NoError(t, err)
	require.Empty(t, leaked)
}

func TestGoroutineTracker_IgnoresGoroutineBlockedOnReachableChannel(t *testing.T) {
	t.Parallel()

	tracker := testhelpers.TrackGoroutines()
	release := make(chan struct{})
	parked := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		close(parked)
		<-release
	}()
	<-parked

	// The goroutine is blocked, but release is still reachable from this
	// test goroutine, so it can resume and is not a leak.
	leaked, err := tracker.Leaked()
	require.NoError(t, err)
	require.Empty(t, leaked)

	close(release)
	<-exited
}

func TestVerifyNoLeaks_FailsOnLeak(t *testing.T) {
	t.Parallel()

	rec := &tbRecorder{TB: t}
	testhelpers.VerifyNoLeaks(rec)
	require.Len(t, rec.cleanups, 1, "VerifyNoLeaks registers exactly one cleanup")

	// The leaking goroutine may not have parked when the cleanup first runs,
	// so poll the cleanup itself; each run takes a fresh profile.
	leakBlockedOnPrivateChannel()
	require.Eventually(t, func() bool {
		rec.runCleanups()
		return len(rec.reported()) > 0
	}, 5*time.Second, 10*time.Millisecond, "VerifyNoLeaks did not report the leak")

	report := rec.reported()[0]
	require.Contains(t, report, "leaked goroutine stack(s)")
	require.Contains(t, report, "leakBlockedOnPrivateChannel")
}

func TestVerifyNoLeaks_PassesWhenGoroutinesExit(t *testing.T) {
	t.Parallel()

	rec := &tbRecorder{TB: t}
	testhelpers.VerifyNoLeaks(rec)

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			ch := make(chan int, 1)
			ch <- 1
			<-ch
		})
	}
	wg.Wait()

	rec.runCleanups()
	require.Empty(t, rec.reported(), "no goroutine leaked: %s", strings.Join(rec.reported(), "\n"))
}

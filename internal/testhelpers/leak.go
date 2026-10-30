// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package testhelpers

import (
	"bytes"
	"context"
	"fmt"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// leakLabelKey is the pprof label that tags the goroutines a
// [GoroutineTracker] is responsible for.
const leakLabelKey = "testhelpers.leakcheck"

// leakTrackerSeq hands out a process-unique label value per tracker.
var leakTrackerSeq atomic.Uint64

// GoroutineTracker finds leaked goroutines started, directly or
// transitively, by the goroutine that created it. A goroutine is leaked when
// the Go runtime's goroutineleak profile reports it: it is blocked on a
// channel or sync primitive that no running goroutine can reach, so it can
// never resume.
//
// Attribution works through pprof labels, which new goroutines inherit from
// their creator: the tracker labels its creating goroutine with a unique
// value and only reports leaked goroutines carrying that value. Trackers in
// parallel tests therefore never see each other's goroutines, and a
// goroutine leaked earlier in the process does not fail later tests.
type GoroutineTracker struct {
	label string
}

// TrackGoroutines labels the calling goroutine and returns a tracker for the
// goroutines it starts from now on. It replaces any pprof labels already set
// on the calling goroutine. Goroutines started before the call, or by
// goroutines that do not descend from the caller (for example a background
// loop started by an earlier test), are not tracked.
func TrackGoroutines() *GoroutineTracker {
	label := strconv.FormatUint(leakTrackerSeq.Add(1), 10)
	pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), pprof.Labels(leakLabelKey, label)))
	return &GoroutineTracker{label: label}
}

// Leaked collects the goroutineleak profile, which runs a leak-detecting
// garbage collection, and returns one entry per distinct leaked stack that
// carries the tracker's label. Each entry is the profile record: the
// goroutine count, the labels and the symbolized stack.
//
// Goroutines that are still running, or blocked on something another live
// goroutine can reach, are not leaked and are not reported, so Leaked never
// reports a goroutine that could still finish; a goroutine that has not yet
// parked when Leaked runs is missed instead.
func (g *GoroutineTracker) Leaked() ([]string, error) {
	var buf bytes.Buffer
	if err := pprof.Lookup("goroutineleak").WriteTo(&buf, 1); err != nil {
		return nil, fmt.Errorf("write goroutineleak profile: %w", err)
	}

	// The debug=1 text format is a "goroutineleak profile: total N" header
	// line followed by blank-line separated records.
	_, records, _ := strings.Cut(buf.String(), "\n")
	needle := strconv.Quote(leakLabelKey) + ":" + strconv.Quote(g.label)
	var leaked []string
	for record := range strings.SplitSeq(records, "\n\n") {
		if strings.Contains(record, needle) {
			leaked = append(leaked, strings.TrimSpace(record))
		}
	}
	return leaked, nil
}

// VerifyNoLeaks fails tb if any goroutine started by the test after this call
// is leaked when the test finishes. It labels the calling goroutine (see
// [TrackGoroutines]) and registers a [testing.TB.Cleanup] that reports every
// leaked stack. Call it first in the test, so its cleanup runs after the
// test's own cleanups have released whatever they release.
//
// The check is precise for the test's own goroutines even under
// [testing.T.Parallel]. It does not detect goroutines that are merely slow
// to exit, nor goroutines blocked on a channel or primitive that is still
// reachable, such as one stored in a package variable.
func VerifyNoLeaks(tb testing.TB) {
	tb.Helper()
	tracker := TrackGoroutines()
	tb.Cleanup(func() {
		leaked, err := tracker.Leaked()
		if err != nil {
			tb.Errorf("goroutine leak check: %v", err)
			return
		}
		if len(leaked) > 0 {
			tb.Errorf("%d leaked goroutine stack(s):\n\n%s", len(leaked), strings.Join(leaked, "\n\n"))
		}
	})
}

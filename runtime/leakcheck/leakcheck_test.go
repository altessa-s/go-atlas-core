// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package leakcheck_test

import (
	"flag"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/leakcheck"
)

// Re-execute the deliberate leak in a child so repeated runs and other tests
// in this process retain a clean goroutine profile.
func TestCheck(t *testing.T) {
	t.Parallel()
	require.NoError(t, leakcheck.Check())
	if pprof.Lookup("goroutineleak") == nil {
		t.Skip("profile unavailable")
	}
	if os.Getenv("CORE_LEAKCHECK_CHILD") != "1" {
		args := []string{"-test.run=^TestCheck$", "-test.count=1"}
		// Include child execution in the parent's coverage report when instrumented.
		if dir := flag.Lookup("test.gocoverdir"); dir != nil && dir.Value.String() != "" {
			args = append(args, "-test.gocoverdir="+dir.Value.String())
		}
		cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
		cmd.Env = append(os.Environ(), "CORE_LEAKCHECK_CHILD=1")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
		require.NoError(t, leakcheck.Check(), "child leak must not affect the parent")
		return
	}
	leakPrivateChannel()
	// Real detecting GC cannot run inside a synctest bubble.
	for range 100 {
		runtime.Gosched()
		err := leakcheck.Check()
		if err != nil {
			require.Contains(t, err.Error(), "leakPrivateChannel")
			require.Contains(t, err.Error(), "leaked goroutines")
			return
		}
	}
	t.Fatal("deliberate leak was not reported")
}

func leakPrivateChannel() {
	ch := make(chan struct{})
	go func() { <-ch }()
}

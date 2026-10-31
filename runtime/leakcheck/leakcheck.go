// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package leakcheck

import (
	"bytes"
	"fmt"
	"runtime/pprof"
)

// Check reports permanently blocked goroutines and their stacks. It runs the
// profile's detecting garbage collection. An unavailable or empty profile is
// successful. Reachable blocked goroutines that could still resume are not leaks.
func Check() error {
	if profile := pprof.Lookup("goroutineleak"); profile != nil {
		var stacks bytes.Buffer
		if err := profile.WriteTo(&stacks, 1); err != nil {
			return fmt.Errorf("goroutine leak profile: %w", err)
		}
		if n := profile.Count(); n > 0 {
			return fmt.Errorf("%d leaked goroutines:\n%s", n, stacks.String())
		}
	}
	return nil
}

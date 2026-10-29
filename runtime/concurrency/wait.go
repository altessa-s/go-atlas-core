// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package concurrency

import (
	"context"
	"sync"
)

// WaitContext waits for wg until ctx ends and returns ctx's error if it ended
// first. The wait itself goes on in a goroutine until wg is done, so a stop
// method can honor its context while the work it stopped winds down.
func WaitContext(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

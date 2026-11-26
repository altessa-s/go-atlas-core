// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package panics

import (
	"fmt"
	"runtime"
	"strings"
)

// RecoveredError wraps a caller-supplied cause and records where a panic
// occurred. It never retains the recovered value.
type RecoveredError struct {
	cause error
	stack string
}

// Error returns the message of the supplied cause.
func (e *RecoveredError) Error() string { return e.cause.Error() }

// Unwrap returns the supplied cause for [errors.Is] and [errors.As].
func (e *RecoveredError) Unwrap() error { return e.cause }

// StackTrace returns the captured function names and source locations.
func (e *RecoveredError) StackTrace() string { return e.stack }

// RecoverError calls fn. On panic, it returns a [*RecoveredError] wrapping the
// supplied non-nil cause. Recovery discards the panic value and bypasses global
// handlers, logging, and re-panic settings.
func RecoverError(cause error, fn func() error) (err error) {
	completed := false

	defer func() {
		if !completed {
			// A completion flag also covers panic(nil) with GODEBUG=panicnil=1.
			_ = recover()
			err = &RecoveredError{cause: cause, stack: captureFrames()}
		}
	}()

	err = fn()
	completed = true

	return err
}

func captureFrames() string {
	const maxFrames = 64

	var pcs [maxFrames]uintptr

	// Skip runtime.Callers, captureFrames, and the deferred recovery function.
	const skipFrames = 3

	n := runtime.Callers(skipFrames, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var stack strings.Builder

	for {
		frame, more := frames.Next()
		fmt.Fprintf(&stack, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)

		if !more {
			break
		}
	}

	return stack.String()
}

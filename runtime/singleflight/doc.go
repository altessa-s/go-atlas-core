// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package singleflight deduplicates concurrent work using comparable keys.
//
// WithLifecycle opts into parent and last-waiter cancellation. The group timeout
// still applies and cannot be disabled; set WithTimeout explicitly when migrating
// work that previously had no execution deadline. For effectively unbounded runs
// like the internal flight helper's, combine WithLifecycle(parent) with
// WithTimeout(time.Duration(math.MaxInt64)). Parent and last-waiter cancellation
// still apply; the group deadline is about 292 years away, not absent.
// Close stops admission; Wait implies Close and drains work. Forget detaches a
// key without canceling its work.
// DoShared reports whether other callers joined. WithPanicAsError can redact
// panic values; by default waiting callers re-panic.
//
// # Admission and cancellation
//
// Distinct keys are admitted before goroutines are created. Overload is reported
// immediately; callers sharing a key wait independently on their own contexts.
//
//	g := singleflight.New[string, string]()
//	value, err := g.Do(ctx, "key", func(work context.Context) (string, error) {
//		return fetch(work, "key")
//	})
package singleflight

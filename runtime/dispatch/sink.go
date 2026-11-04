// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package dispatch

import "context"

// Sink is the destination for batched items processed by Engine. The
// delivery guarantee depends on the mode:
//
//   - With WithWAL, delivery is at-least-once: a failed batch stays in the
//     WAL and is retried until it is delivered or the engine shuts down, and
//     a single item may appear in StoreBatch more than once after a
//     crash-and-replay sequence. Implementations should be idempotent on the
//     records' natural identity (e.g. event ID).
//   - Without WithWAL, delivery is best effort: a failed batch is retried up
//     to WithRetryAttempts times and then dropped. The failure is reported
//     through Observer.BatchStoreFailed and the last sink error is joined
//     into the error returned by Engine.Shutdown; buffered items are lost on
//     a crash.
type Sink[T any] interface {
	// StoreBatch persists a batch of items. The batch is non-empty.
	StoreBatch(ctx context.Context, items []T) error
}

// Codec serializes items for WAL durability and decodes them on recovery.
// Implementations should be deterministic and stable across process
// restarts; encode and decode must round-trip exactly.
type Codec[T any] interface {
	Encode(item T) ([]byte, error)
	Decode(b []byte) (T, error)
}

// DropHandler is invoked when an item is dropped at submit time due to a
// full in-memory buffer (and back-pressure mode is disabled).
type DropHandler[T any] func(item T)

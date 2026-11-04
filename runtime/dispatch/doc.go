// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

// Package dispatch provides a generic, non-blocking, batching dispatch
// engine with optional crash-safe persistence backed by
// [github.com/altessa-s/go-atlas-core/io/wal].
//
// The engine is designed for high-throughput "fire-and-forget" producers
// (audit logs, request access logs, telemetry) where the hot path must not
// block on I/O but events must still survive process crashes.
//
// # Hot path
//
// Without WAL, Engine.Submit only sends the item to an in-memory channel and
// returns. With WAL enabled, Submit additionally serializes the item via the
// configured Codec, appends it to the active WAL segment (page-cache write,
// no fsync) and wakes a worker; durable workers claim records straight from
// the WAL rather than from the channel. A background goroutine fsyncs
// the active segment on the interval configured via wal.WithFsyncInterval
// (group commit), bounding the loss window after a crash.
//
// # Workers
//
// Without WAL, worker goroutines drain the channel, build batches up to
// BatchSize or FlushInterval, and call Sink.StoreBatch. With WAL, durable
// workers claim up to BatchSize unacknowledged records straight from the WAL
// when woken or on FlushInterval, decode them via Codec and call
// Sink.StoreBatch; on success the records are acked, and once all records in
// a sealed segment are acked the segment file is removed.
//
// # Recovery
//
// On Engine.Start, the WAL directory is scanned and every valid record is
// decoded via Codec to reject undecodable data early before Submit is opened;
// the records stay in the WAL, and durable workers claim them after Start like
// any other pending record. Torn writes
// (partial records left after a crash) are detected by CRC32 and discarded.
//
// # Durability guarantees
//
// With WAL, the sink receives at-least-once delivery:
//
//   - Graceful Shutdown: drains the queue and waits for the sink, then closes
//     the WAL. Acked segments are removed; nothing is lost.
//   - Sink failure: a failed batch stays in the WAL and is retried in capped
//     backoff rounds until it is delivered or Shutdown gives up, which then
//     reports ErrBacklog; a new engine on the same directory replays it.
//   - Crash (SIGKILL/panic/OOM): records that reached the most recent fsync
//     survive and are replayed on next start. The loss window is bounded by
//     FsyncInterval.
//
// Without WAL, delivery is best effort: a failed batch is retried up to
// RetryAttempts times and then dropped, reported through
// [Observer.BatchStoreFailed] and the error returned by Engine.Shutdown, and
// buffered items are lost on a crash.
//
// # Disabling the WAL
//
// Pass no WithWAL option to keep the engine purely in-memory. In that mode
// the engine is equivalent to a generic batching dispatcher with no I/O on
// the submit path; the Codec field is not required. WithWAL with an empty
// directory or a nil codec does not fall back to this mode: NewEngine fails
// with ErrWALDirRequired or ErrCodecRequired.
//
// # Performance
//
// Submit is allocation-free on the no-WAL hot path: items are sent
// through a buffered channel, and back-pressure is
// configurable via WithBackPressure. With WAL enabled, each Submit pays
// one codec encode plus a mutex-serialized file.Write inside the WAL
// package — see the Performance section of
// [github.com/altessa-s/go-atlas-core/io/wal] for details on the
// Append mutex trade-off.
//
// Volatile workers reuse their batch slices via zero-length
// reslicing (batch[:0]) across flushes, so the steady-state worker cost
// is one ticker interrupt per FlushInterval plus the per-batch sink call.
// Encode errors and full-buffer drops both fall through e.dropItem,
// reporting [Observer.ItemDropped] and firing the OnDrop callback, so
// observability is uniform across drop causes.
//
// # Observability
//
// The engine reports its events through an [Observer] given with
// [WithObserver]; without one it uses [NoopObserver]. Failed sink attempts are
// reported at item granularity ([Observer.BatchStoreFailed] receives the batch
// size), matching [Observer.ItemEnqueued], so consumers can reason about loss
// rate from a single pair of counters. go-atlas ships a metrics adapter.
package dispatch

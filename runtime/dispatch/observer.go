// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package dispatch

import "time"

// Observer receives the events an [Engine] emits, typically to export them as
// metrics. Methods are called on the Submit and worker hot paths and must be
// cheap, safe for concurrent use and non-blocking.
//
// Embed [NoopObserver] to implement only the events of interest and stay
// compatible when new events are added.
type Observer interface {
	// ItemEnqueued reports an item admitted to the buffer or the WAL.
	ItemEnqueued()
	// ItemDropped reports an item rejected by a full buffer, an encode
	// failure or a WAL append failure.
	ItemDropped()
	// ItemsReplayed reports the number of records recovered from the WAL
	// on Start.
	ItemsReplayed(n int)
	// EncodeFailed reports a codec encode failure on Submit.
	EncodeFailed()
	// WALAppendFailed reports a WAL append failure on Submit.
	WALAppendFailed()
	// BatchStoreFailed reports a failed sink attempt for a batch of the
	// given size. It is called once per failed attempt, retries included.
	BatchStoreFailed(items int)
	// BatchDone reports how long a batch spent in the sink, retries
	// included, whether or not it was delivered.
	BatchDone(d time.Duration)
	// WorkerStarted and WorkerStopped bracket the life of a worker.
	WorkerStarted()
	WorkerStopped()
	// WALSize reports the bytes currently held by the WAL across all
	// segments. It is sampled once per flushed batch and once per failed
	// sink attempt.
	WALSize(bytes int64)
}

// NoopObserver is an [Observer] that ignores every event. It is the default
// when no [WithObserver] is given.
type NoopObserver struct{}

var _ Observer = NoopObserver{}

// ItemEnqueued implements [Observer].
func (NoopObserver) ItemEnqueued() {}

// ItemDropped implements [Observer].
func (NoopObserver) ItemDropped() {}

// ItemsReplayed implements [Observer].
func (NoopObserver) ItemsReplayed(int) {}

// EncodeFailed implements [Observer].
func (NoopObserver) EncodeFailed() {}

// WALAppendFailed implements [Observer].
func (NoopObserver) WALAppendFailed() {}

// BatchStoreFailed implements [Observer].
func (NoopObserver) BatchStoreFailed(int) {}

// BatchDone implements [Observer].
func (NoopObserver) BatchDone(time.Duration) {}

// WorkerStarted implements [Observer].
func (NoopObserver) WorkerStarted() {}

// WorkerStopped implements [Observer].
func (NoopObserver) WorkerStopped() {}

// WALSize implements [Observer].
func (NoopObserver) WALSize(int64) {}

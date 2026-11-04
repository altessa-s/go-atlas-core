// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package dispatch_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/io/wal"
	"github.com/altessa-s/go-atlas-core/runtime/dispatch"
)

// recordingObserver counts the events an Engine reports.
type recordingObserver struct {
	dispatch.NoopObserver

	enqueued, dropped, encodeFailed, batches, workers atomic.Int64
	walSize                                           atomic.Int64
	storeFailed, storeFailedItems, replayed           atomic.Int64
	walAppendFailed, batchNanos                       atomic.Int64
}

func (o *recordingObserver) ItemEnqueued() { o.enqueued.Add(1) }
func (o *recordingObserver) ItemDropped()  { o.dropped.Add(1) }
func (o *recordingObserver) EncodeFailed() { o.encodeFailed.Add(1) }
func (o *recordingObserver) BatchDone(d time.Duration) {
	o.batches.Add(1)
	o.batchNanos.Add(int64(d))
}
func (o *recordingObserver) BatchStoreFailed(items int) {
	o.storeFailed.Add(1)
	o.storeFailedItems.Add(int64(items))
}
func (o *recordingObserver) ItemsReplayed(n int) { o.replayed.Add(int64(n)) }
func (o *recordingObserver) WALAppendFailed()    { o.walAppendFailed.Add(1) }
func (o *recordingObserver) WorkerStarted()      { o.workers.Add(1) }
func (o *recordingObserver) WorkerStopped()      { o.workers.Add(-1) }
func (o *recordingObserver) WALSize(bytes int64) { o.walSize.Store(bytes) }

// TestEngine_ObserverEvents pins the events a volatile engine reports through
// its Observer: admission, flushed batches and the worker lifecycle.
func TestEngine_ObserverEvents(t *testing.T) {
	t.Parallel()

	obs := &recordingObserver{}
	e, err := dispatch.NewEngine[int](&fakeSink{},
		dispatch.WithObserver[int](obs),
		dispatch.WithWorkers[int](1))
	require.NoError(t, err)
	require.NoError(t, e.Start())
	waitFor(t, 2*time.Second, func() bool { return obs.workers.Load() == 1 }, "worker never started")

	require.True(t, e.Submit(1))
	require.NoError(t, e.Shutdown(t.Context()))

	require.EqualValues(t, 1, obs.enqueued.Load())
	require.EqualValues(t, 1, obs.batches.Load())
	require.Zero(t, obs.dropped.Load())
	require.Zero(t, obs.workers.Load())
}

// failingCodec rejects every item on Encode.
type failingCodec struct{}

func (failingCodec) Encode(int) ([]byte, error) { return nil, errors.New("encode") }
func (failingCodec) Decode([]byte) (int, error) { return 0, errors.New("decode") }

// TestEngine_ObserverEncodeFailure pins that an item the codec cannot encode
// is reported as both an encode failure and a drop.
func TestEngine_ObserverEncodeFailure(t *testing.T) {
	t.Parallel()

	obs := &recordingObserver{}
	e, err := dispatch.NewEngine[int](&fakeSink{},
		dispatch.WithWAL[int](filepath.Join(t.TempDir(), "wal"), failingCodec{}),
		dispatch.WithObserver[int](obs))
	require.NoError(t, err)
	require.NoError(t, e.Start())

	require.False(t, e.Submit(1))
	require.NoError(t, e.Shutdown(t.Context()))

	require.EqualValues(t, 1, obs.encodeFailed.Load())
	require.EqualValues(t, 1, obs.dropped.Load())
	require.Zero(t, obs.enqueued.Load())
}

// TestEngine_NilObserver pins that WithObserver(nil) falls back to the no-op
// observer instead of panicking on the first event.
func TestEngine_NilObserver(t *testing.T) {
	t.Parallel()

	e, err := dispatch.NewEngine[int](&fakeSink{}, dispatch.WithObserver[int](nil))
	require.NoError(t, err)
	require.NoError(t, e.Start())
	require.True(t, e.Submit(1))
	require.NoError(t, e.Shutdown(t.Context()))
}

// TestEngine_ShutdownWithoutStart pins that an engine which was constructed but
// never started can still be shut down.
//
// Shutdown unconditionally invokes the cancel func for the engine's lifecycle
// context. While that context was created in Start rather than in NewEngine,
// the field was nil on this path and Shutdown panicked — reachable from any
// caller that builds an engine and then bails out before starting it, which is
// exactly what a failed dependency wire-up does.
func TestEngine_ShutdownWithoutStart(t *testing.T) {
	t.Parallel()

	e, err := dispatch.NewEngine[int](&fakeSink{})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		require.NoError(t, e.Shutdown(t.Context()))
	})
}

// TestEngine_WALBytesGaugeUpdatedOnFlush pins that moving the WAL size sample
// off the Submit path did not stop it from being reported.
//
// Stats takes the same mutex as Append, so sampling it per item doubled the
// WAL lock acquisitions on the hot path. It now runs once per flushed batch,
// which is still often enough for a size gauge — but only if it actually runs.
func TestEngine_WALSizeReportedOnFlush(t *testing.T) {
	t.Parallel()

	obs := &recordingObserver{}
	sink := &fakeSink{}
	e, err := dispatch.NewEngine[int](sink,
		dispatch.WithWAL[int](filepath.Join(t.TempDir(), "wal"), intCodec{}),
		dispatch.WithObserver[int](obs),
		dispatch.WithFlushInterval[int](20*time.Millisecond))
	require.NoError(t, err)
	require.NoError(t, e.Start())

	require.True(t, e.Submit(1))

	require.Eventually(t, func() bool {
		return obs.walSize.Load() > 0
	}, 2*time.Second, 20*time.Millisecond, "wal_bytes was never published")

	stopCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, e.Shutdown(stopCtx))
}

// TestEngine_RetryWithJitterStillDelivers guards the jitter wiring. core/retry
// owns the distribution (TestExponential_Jitter); what can break here is the
// value handed to it — a jitter that inflates the backoff past the test's
// patience, or one that never reaches the retry loop at all.
func TestEngine_RetryWithJitterStillDelivers(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{failN: 2}
	e, err := dispatch.NewEngine[int](sink,
		dispatch.WithRetryBackoff[int](5*time.Millisecond),
		dispatch.WithRetryJitter[int](1.0),
		dispatch.WithFlushInterval[int](10*time.Millisecond))
	require.NoError(t, err)
	require.NoError(t, e.Start())

	require.True(t, e.Submit(42))

	// Wait for the retry sequence to finish before shutting down. Shutdown
	// short-circuits the backoff sleep into a single last-ditch store, which
	// would consume one of the sink's scripted failures and make the outcome
	// depend on timing rather than on the retry logic under test.
	require.Eventually(t, func() bool { return len(sink.snapshot()) == 1 },
		5*time.Second, 10*time.Millisecond, "item was never delivered")

	stopCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, e.Shutdown(stopCtx))

	require.Equal(t, []int{42}, sink.snapshot(), "item must survive jittered retries")
}

// TestEngine_ConcurrentStartAndSubmit is a race-detector regression for the
// publication of the WAL handle.
//
// Submit used to gate on the started flag, which Start sets by CAS before it
// opens the WAL. That gave Submit no happens-before edge to the later
// e.log assignment: the race detector reported a data race on the field, and
// the practical consequence was worse than the report — a Submit landing in
// that window observed a nil log and silently bypassed durability, losing the
// one guarantee the WAL exists to provide. Submit now gates on ready, which is
// published after the handle is in place.
//
// Only meaningful under -race; without it this merely exercises the path.
func TestEngine_ConcurrentStartAndSubmit(t *testing.T) {
	t.Parallel()

	const attempts = 20

	for i := range attempts {
		e, err := dispatch.NewEngine[int](&fakeSink{},
			dispatch.WithWAL[int](filepath.Join(t.TempDir(), "wal"), intCodec{}))
		require.NoError(t, err)

		var (
			wg       sync.WaitGroup
			startErr error
		)
		wg.Go(func() { startErr = e.Start() })
		wg.Go(func() { e.Submit(i) })
		wg.Wait()
		require.NoError(t, startErr)

		stopCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		require.NoError(t, e.Shutdown(stopCtx))
		cancel()
	}
}

// TestEngine_ObserverRetryEvents pins that every failed sink attempt is
// reported with the batch size, and that BatchDone fires once per batch with
// a duration that includes the retry backoff.
func TestEngine_ObserverRetryEvents(t *testing.T) {
	t.Parallel()

	const backoff = 20 * time.Millisecond
	obs := &recordingObserver{}
	e, err := dispatch.NewEngine[int](&fakeSink{failN: 2},
		dispatch.WithObserver[int](obs),
		dispatch.WithWorkers[int](1),
		dispatch.WithBatchSize[int](3),
		dispatch.WithRetryAttempts[int](3),
		dispatch.WithRetryBackoff[int](backoff),
		dispatch.WithRetryJitter[int](0))
	require.NoError(t, err)
	require.NoError(t, e.Start())

	for i := range 3 {
		require.True(t, e.Submit(i))
	}
	waitFor(t, 5*time.Second, func() bool { return obs.batches.Load() == 1 }, "batch never completed")
	require.NoError(t, e.Shutdown(t.Context()))

	require.EqualValues(t, 2, obs.storeFailed.Load(), "one event per failed attempt")
	require.EqualValues(t, 6, obs.storeFailedItems.Load(), "each failed attempt reports the batch size")
	require.EqualValues(t, 1, obs.batches.Load())
	require.GreaterOrEqual(t, time.Duration(obs.batchNanos.Load()), backoff, "BatchDone includes the retry backoff")
}

// TestEngine_WALSizeReportedDuringOutage pins that the WAL size is refreshed
// while the sink keeps failing, so the gauge follows a growing backlog.
func TestEngine_WALSizeReportedDuringOutage(t *testing.T) {
	t.Parallel()

	obs := &recordingObserver{}
	e, err := dispatch.NewEngine[int](&fakeSink{failN: 1 << 30},
		dispatch.WithWAL[int](filepath.Join(t.TempDir(), "wal"), intCodec{}),
		dispatch.WithObserver[int](obs),
		dispatch.WithRetryBackoff[int](time.Millisecond),
		dispatch.WithRetryMaxBackoff[int](5*time.Millisecond),
		dispatch.WithFlushInterval[int](10*time.Millisecond))
	require.NoError(t, err)
	require.NoError(t, e.Start())

	require.True(t, e.Submit(1))
	waitFor(t, 2*time.Second, func() bool { return obs.walSize.Load() > 0 }, "wal size never reported during the outage")
	require.Zero(t, obs.batches.Load(), "no batch was delivered")

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_ = e.Shutdown(ctx) // The undelivered backlog is expected.
}

// TestEngine_ObserverReplayAndAppendFailure pins ItemsReplayed for records a
// previous engine left in the WAL, and WALAppendFailed when the WAL is full.
func TestEngine_ObserverReplayAndAppendFailure(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "wal")
	first, err := dispatch.NewEngine[int](&fakeSink{failN: 1 << 30},
		dispatch.WithWAL[int](dir, intCodec{}),
		dispatch.WithRetryBackoff[int](time.Millisecond))
	require.NoError(t, err)
	require.NoError(t, first.Start())
	require.True(t, first.Submit(1))
	require.True(t, first.Submit(2))
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_ = first.Shutdown(ctx) // Leaves both records in the WAL.

	obs := &recordingObserver{}
	second, err := dispatch.NewEngine[int](&fakeSink{},
		dispatch.WithWAL[int](dir, intCodec{}, wal.WithMaxBytes(1)),
		dispatch.WithObserver[int](obs))
	require.NoError(t, err)
	require.NoError(t, second.Start())
	require.EqualValues(t, 2, obs.replayed.Load())

	require.False(t, second.Submit(3), "the WAL is over its one-byte cap")
	require.EqualValues(t, 1, obs.walAppendFailed.Load())
	require.EqualValues(t, 1, obs.dropped.Load())
	require.NoError(t, second.Shutdown(t.Context()))
}

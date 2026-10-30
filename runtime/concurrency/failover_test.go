// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package concurrency_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/concurrency"
)

var (
	errFetch   = errors.New("upstream down")
	errStale   = errors.New("stale")
	errInvalid = errors.New("invalid")
)

type countingSource struct {
	name  string
	calls int
	fn    func(ctx context.Context) (string, error)
}

func (s *countingSource) source() concurrency.Source[string] {
	return concurrency.Source[string]{Name: s.name, Fetch: func(ctx context.Context) (string, error) {
		s.calls++
		return s.fn(ctx)
	}}
}

func okSource(name, value string) *countingSource {
	return &countingSource{name: name, fn: func(context.Context) (string, error) { return value, nil }}
}

func errSource(name string, err error) *countingSource {
	return &countingSource{name: name, fn: func(context.Context) (string, error) { return "", err }}
}

func sources(cs ...*countingSource) []concurrency.Source[string] {
	out := make([]concurrency.Source[string], 0, len(cs))
	for _, c := range cs {
		out = append(out, c.source())
	}
	return out
}

func rejectByValue(_ string, v string) error {
	switch v {
	case "stale":
		return fmt.Errorf("%w: old date", errStale)
	case "invalid":
		return errInvalid
	default:
		return nil
	}
}

func TestFirst(t *testing.T) {
	t.Parallel()

	t.Run("first source admitted", func(t *testing.T) {
		t.Parallel()
		p1, p2 := okSource("p1", "a"), okSource("p2", "b")
		res, err := concurrency.First(t.Context(), sources(p1, p2), nil)
		require.NoError(t, err)
		require.Equal(t, "p1", res.Source)
		require.Equal(t, "a", res.Value)
		require.Empty(t, res.Skipped)
		require.Zero(t, p2.calls)
	})

	t.Run("fetch failure and rejection then success", func(t *testing.T) {
		t.Parallel()
		p1, p2, p3 := errSource("p1", errFetch), okSource("p2", "stale"), okSource("p3", "fresh")
		res, err := concurrency.First(t.Context(), sources(p1, p2, p3), rejectByValue)
		require.NoError(t, err)
		require.Equal(t, "p3", res.Source)
		require.Equal(t, "fresh", res.Value)
		require.Len(t, res.Skipped, 2)
		require.Equal(t, "p1", res.Skipped[0].Source)
		require.ErrorIs(t, res.Skipped[0], errFetch)
		require.Equal(t, "p2", res.Skipped[1].Source)
		require.ErrorIs(t, res.Skipped[1], errStale)
	})

	t.Run("accept receives the source name", func(t *testing.T) {
		t.Parallel()
		var seen []string
		accept := func(name, _ string) error {
			seen = append(seen, name)
			if name == "p1" {
				return errInvalid
			}
			return nil
		}
		res, err := concurrency.First(t.Context(), sources(okSource("p1", "x"), okSource("p2", "y")), accept)
		require.NoError(t, err)
		require.Equal(t, []string{"p1", "p2"}, seen)
		require.Equal(t, "y", res.Value)
	})

	t.Run("all sources skipped", func(t *testing.T) {
		t.Parallel()
		p1, p2, p3 := errSource("p1", errFetch), okSource("p2", "stale"), okSource("p3", "invalid")
		res, err := concurrency.First(t.Context(), sources(p1, p2, p3), rejectByValue)
		require.ErrorIs(t, err, concurrency.ErrAllFailed)
		require.ErrorIs(t, err, errFetch)
		require.ErrorIs(t, err, errStale)
		require.ErrorIs(t, err, errInvalid)
		se, ok := errors.AsType[*concurrency.SourceError](err)
		require.True(t, ok)
		require.Equal(t, "p1", se.Source)
		require.Len(t, res.Skipped, 3)
		require.Empty(t, res.Source)
		require.Empty(t, res.Value)
	})

	t.Run("no sources", func(t *testing.T) {
		t.Parallel()
		res, err := concurrency.First[string](t.Context(), nil, nil)
		require.ErrorIs(t, err, concurrency.ErrAllFailed)
		require.Nil(t, res.Skipped)
	})

	t.Run("context canceled before the first source", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		p1 := okSource("p1", "a")
		res, err := concurrency.First(ctx, sources(p1), nil)
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, concurrency.ErrAllFailed)
		require.Empty(t, res.Skipped)
		require.Zero(t, p1.calls)
	})

	t.Run("context canceled during a fetch", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		p1 := errSource("p1", errFetch)
		p2 := &countingSource{name: "p2", fn: func(ctx context.Context) (string, error) {
			cancel()
			return "", ctx.Err()
		}}
		p3 := okSource("p3", "ok")
		res, err := concurrency.First(ctx, sources(p1, p2, p3), nil)
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, concurrency.ErrAllFailed)
		require.Len(t, res.Skipped, 1)
		require.Equal(t, "p1", res.Skipped[0].Source)
		require.Zero(t, p3.calls)
	})

	t.Run("per-source timeout with live context is a skip", func(t *testing.T) {
		t.Parallel()
		p1 := errSource("p1", fmt.Errorf("http: %w", context.DeadlineExceeded))
		res, err := concurrency.First(t.Context(), sources(p1, okSource("p2", "ok")), nil)
		require.NoError(t, err)
		require.Equal(t, "p2", res.Source)
		require.Len(t, res.Skipped, 1)
		require.ErrorIs(t, res.Skipped[0], context.DeadlineExceeded)
	})
}

func TestSourceErrorNil(t *testing.T) {
	t.Parallel()
	var se *concurrency.SourceError
	require.Empty(t, se.Error())
	require.NoError(t, se.Unwrap())
	require.Equal(t, "source p1: upstream down", (&concurrency.SourceError{Source: "p1", Err: errFetch}).Error())
}

func TestCanceled(t *testing.T) {
	t.Parallel()

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()
	expiredCtx, cancelDeadline := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	t.Cleanup(cancelDeadline)

	cases := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{"canceled with context.Canceled", canceledCtx, context.Canceled, true},
		{"deadline passed with DeadlineExceeded", expiredCtx, context.DeadlineExceeded, true},
		{"canceled with wrapped context.Canceled", canceledCtx, fmt.Errorf("upsert: %w", context.Canceled), true},
		{"alive with a source timeout", t.Context(), context.DeadlineExceeded, false},
		{
			"canceled with ErrAllFailed wrapping DeadlineExceeded", canceledCtx,
			errors.Join(concurrency.ErrAllFailed, &concurrency.SourceError{Source: "p1", Err: context.DeadlineExceeded}), false,
		},
		{"canceled with an operational error", canceledCtx, errFetch, false},
		{"canceled with nil error", canceledCtx, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, concurrency.Canceled(tc.ctx, tc.err))
		})
	}
}

// A caller cancellation observed by the accept callback ends First with the
// context error itself, not as a skipped source joined into ErrAllFailed.
func TestFirstCanceledDuringAccept(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	sources := []concurrency.Source[int]{{Name: "only", Fetch: func(context.Context) (int, error) { return 1, nil }}}
	res, err := concurrency.First(ctx, sources, func(string, int) error {
		cancel()
		return ctx.Err()
	})
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, concurrency.ErrAllFailed)
	require.Empty(t, res.Skipped)
	require.True(t, concurrency.Canceled(ctx, err))
}

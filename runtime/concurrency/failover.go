// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package concurrency

import (
	"context"
	"errors"
	"fmt"
)

// ErrAllFailed is joined into the error [First] returns when every source was
// skipped, or returned alone when there are no sources.
var ErrAllFailed = errors.New("concurrency: all sources failed")

// Source is one named candidate tried by [First].
type Source[T any] struct {
	// Name identifies the source in [SourceError] and [Result].
	Name string
	// Fetch produces the candidate value. It must not be nil.
	Fetch func(ctx context.Context) (T, error)
}

// SourceError attributes one skipped source: a Fetch error or an accept rejection.
type SourceError struct {
	Source string
	Err    error
}

// Error implements [error].
func (e *SourceError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("source %s: %v", e.Source, e.Err)
}

// Unwrap returns the underlying source error.
func (e *SourceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Result is the outcome of [First].
type Result[T any] struct {
	// Value is the admitted value; zero when First returns an error.
	Value T
	// Source is the Name of the source that produced Value; empty when First returns an error.
	Source string
	// Skipped lists every source skipped before the admitted one (or all of them), in order.
	Skipped []*SourceError
}

// First tries sources in order and returns the first value that accept admits.
//
// A source is skipped when Fetch fails or accept rejects its value; each skip appends one *SourceError to Result.Skipped.
// accept receives the source name and the fetched value and carries the caller's admissibility policy; a nil accept admits
// every successfully fetched value. The sources are tried sequentially: a later source is called only after every earlier one
// has been skipped.
//
// Result.Skipped is filled on every path, so callers can count failures also when First fails:
//   - success: nil error; len(Skipped) is the number of sources skipped before Result.Source;
//   - every source skipped: errors.Join(ErrAllFailed, Skipped[0], ..., Skipped[n-1]). errors.Is matches ErrAllFailed and every
//     cause; errors.As finds the first *SourceError;
//   - no sources: ErrAllFailed alone, Skipped nil;
//   - ctx done (checked before each source and after a failed Fetch or a rejected value): ctx.Err() is returned as-is, not joined with
//     ErrAllFailed. The interrupted source is not in Skipped and no further source is called.
//
// A Fetch error wrapping context.DeadlineExceeded while ctx is still alive (a per-source timeout) is an ordinary skip.
// Use [Canceled] to tell a cancellation of the caller's context from an operational failure.
func First[T any](ctx context.Context, sources []Source[T], accept func(source string, value T) error) (Result[T], error) {
	var res Result[T]
	if len(sources) == 0 {
		return res, ErrAllFailed
	}
	for _, src := range sources {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		value, err := src.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			res.Skipped = append(res.Skipped, &SourceError{Source: src.Name, Err: err})
			continue
		}
		if accept != nil {
			if err := accept(src.Name, value); err != nil {
				if ctx.Err() != nil {
					return res, ctx.Err()
				}
				res.Skipped = append(res.Skipped, &SourceError{Source: src.Name, Err: err})
				continue
			}
		}
		res.Value = value
		res.Source = src.Name
		return res, nil
	}
	joined := make([]error, 0, 1+len(res.Skipped))
	joined = append(joined, ErrAllFailed)
	for _, se := range res.Skipped {
		joined = append(joined, se)
	}
	return res, errors.Join(joined...)
}

// Canceled reports whether err ended an operation because ctx is done (shutdown, operator cancel, parent deadline), as opposed
// to an operational failure. All three conditions are required:
//   - ctx.Err() != nil: a source's own timeout while ctx is alive is a skip inside [First], not a cancellation;
//   - !errors.Is(err, ErrAllFailed): the joined error may contain a source timeout that wraps context.DeadlineExceeded;
//   - errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded).
//
// It applies to any error observed under ctx, not only to errors returned by [First].
func Canceled(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() == nil || errors.Is(err, ErrAllFailed) {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

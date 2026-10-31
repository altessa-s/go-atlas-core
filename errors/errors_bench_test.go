// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package errors_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/altessa-s/go-atlas-core/errors"

	std_errors "errors"
)

// BenchmarkProvider measures the happy path — a non-nil cause being wrapped.
// Useful to confirm the nil guard adds no measurable overhead to the common
// path.
func BenchmarkProvider(b *testing.B) {
	cause := std_errors.New("connection refused")
	b.ReportAllocs()
	for b.Loop() {
		_ = errors.Provider("Redis", cause)
	}
}

// BenchmarkProvider_NilFastPath measures the new nil guard path. It should be
// near-zero cost and allocation-free.
func BenchmarkProvider_NilFastPath(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = errors.Provider("Redis", nil)
	}
}

// BenchmarkWrap measures the plain message wrap used on every error return
// path.
func BenchmarkWrap(b *testing.B) {
	cause := std_errors.New("connection refused")
	b.ReportAllocs()
	for b.Loop() {
		_ = errors.Wrap(cause, "dial upstream")
	}
}

// BenchmarkWrapf measures the formatted wrap, which goes through fmt.
func BenchmarkWrapf(b *testing.B) {
	cause := std_errors.New("connection refused")
	b.ReportAllocs()
	for b.Loop() {
		_ = errors.Wrapf(cause, "dial %s:%d", "db.internal", 5432)
	}
}

// BenchmarkWrapOperation measures the structured "failed to X" wrap.
func BenchmarkWrapOperation(b *testing.B) {
	cause := std_errors.New("connection refused")
	b.ReportAllocs()
	for b.Loop() {
		_ = errors.WrapOperation(cause, "load config")
	}
}

// BenchmarkWrapField measures wrapping a validation error with a field name.
func BenchmarkWrapField(b *testing.B) {
	cause := std_errors.New("must not be empty")
	b.ReportAllocs()
	for b.Loop() {
		_ = errors.WrapField(cause, "base_url")
	}
}

// BenchmarkJoinWrap measures tagging a cause with a domain sentinel and then
// matching the sentinel, the pattern callers use to classify errors.
func BenchmarkJoinWrap(b *testing.B) {
	cause := std_errors.New("no rows")
	b.ReportAllocs()
	for b.Loop() {
		err := errors.JoinWrap(errors.ErrNotFound, cause)
		if !std_errors.Is(err, errors.ErrNotFound) {
			b.Fatal("sentinel not matched")
		}
	}
}

// BenchmarkIsContextCanceledOrDeadlineExceeded measures classification of a
// context error buried under several wrap layers, as seen in shutdown paths.
func BenchmarkIsContextCanceledOrDeadlineExceeded(b *testing.B) {
	for _, depth := range []int{1, 8} {
		err := error(context.DeadlineExceeded)
		for i := range depth {
			err = errors.Wrapf(err, "layer %d", i)
		}
		miss := errors.Wrap(std_errors.New("boom"), "layer")
		b.Run(fmt.Sprintf("hit/depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if !errors.IsContextCanceledOrDeadlineExceeded(err) {
					b.Fatal("context error not matched")
				}
			}
		})
		b.Run(fmt.Sprintf("miss/depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if errors.IsContextCanceledOrDeadlineExceeded(miss) {
					b.Fatal("unexpected match")
				}
			}
		})
	}
}

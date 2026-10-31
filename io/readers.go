// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package io

import (
	"errors"
	"io"
	"math"
	"sync"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// ErrReadLimitExceeded is a sentinel error returned by [LimitedReadCloser.Read] when the
// cumulative number of bytes read exceeds the configured limit. Callers can test for this
// condition with [errors.Is].
var ErrReadLimitExceeded = errors.New("read limit exceeded")

// ErrorReader is an [io.Reader] implementation that returns zero bytes read and
// the configured [ErrorReader.Err] on every call to [ErrorReader.Read]. It is useful
// in tests and error-path simulations where a reader must fail deterministically.
// Create instances with [NewErrorReader].
type ErrorReader struct {
	// Err is the error returned by every call to [ErrorReader.Read]. It must be
	// set before the first Read and must not be mutated afterward, as Read
	// accesses it without synchronization.
	Err error
}

// Read always returns 0 bytes read and the configured [ErrorReader.Err]. The
// destination slice is never modified.
func (e *ErrorReader) Read(_ []byte) (int, error) {
	return 0, e.Err
}

// NewErrorReader creates an [ErrorReader] whose [ErrorReader.Read] method always
// returns (0, err). The supplied error is stored as-is and not wrapped.
func NewErrorReader(err error) *ErrorReader {
	return &ErrorReader{Err: err}
}

// LimitedReadCloser wraps an [io.ReadCloser] and enforces a maximum number of bytes
// that may be read. It never delivers more than the configured limit: once the
// source turns out to hold more, Read returns an error wrapping
// [ErrReadLimitExceeded], and every later Read returns it again without data. A
// source of exactly limit bytes reads to [io.EOF] without error. The underlying
// reader's Close method is preserved and delegated to by [LimitedReadCloser.Close].
//
// LimitedReadCloser is safe for concurrent reads: a mutex serializes each read of
// the underlying reader with the update of the running byte total, so the limit
// holds across goroutines. Create instances with [NewLimitedReadCloser].
type LimitedReadCloser struct {
	reader io.Reader
	closer io.Closer
	limit  int64

	mu   sync.Mutex
	read int64 // bytes pulled from the source, at most limit+1; guarded by mu
}

// NewLimitedReadCloser creates a [LimitedReadCloser] that allows at most limit bytes
// to be read from rc. Internally, the underlying reader is wrapped with an
// [io.LimitReader] set to limit+1 so that an over-read can be detected and reported
// as [ErrReadLimitExceeded] rather than a silent truncation at the boundary; that
// extra byte is consumed from rc but never handed to the caller.
func NewLimitedReadCloser(rc io.ReadCloser, limit int64) *LimitedReadCloser {
	var reader io.Reader = rc
	if limit < math.MaxInt64 {
		reader = io.LimitReader(rc, limit+1)
	}
	return &LimitedReadCloser{
		reader: reader,
		closer: rc,
		limit:  limit,
	}
}

// Read reads up to len(p) bytes from the underlying reader into p. It returns the
// number of bytes read and any error encountered. The cumulative count returned
// across calls never exceeds the configured limit: when the source holds more,
// Read returns the bytes up to the limit with an error wrapping
// [ErrReadLimitExceeded]. Bytes of p past the returned count may have been
// overwritten. An underlying read error is preserved when the limit is exceeded.
// Concurrent Read calls are serialized.
func (l *LimitedReadCloser) Read(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	n, err = l.reader.Read(p)

	// remaining is what may still be delivered before this read; it is negative
	// only once an earlier read already detected the overflow.
	remaining := l.limit - l.read
	l.read += int64(n)
	if l.read > l.limit {
		n = int(max(min(remaining, int64(n)), 0))
		return n, errors.Join(coreerrs.Wrapf(ErrReadLimitExceeded, "more than %d bytes available", l.limit), err)
	}

	return n, err
}

// Close delegates to the underlying [io.ReadCloser]'s Close method. If the underlying
// closer is nil, Close returns nil.
func (l *LimitedReadCloser) Close() error {
	if l.closer != nil {
		return l.closer.Close()
	}
	return nil
}

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package io

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrRangeOverrun is returned by [RangeReadSeeker.Read] when the body opened
// by the [RangeOpener] continues past the end of the requested range — a
// source that ignored the range (returning the whole object) or returned more
// than asked. The bytes already delivered for such a range are suspect.
var ErrRangeOverrun = errors.New("ranged body longer than requested range")

// Seek sentinel errors returned by [RangeReadSeeker.Seek]. Callers can test for
// these conditions with [errors.Is].
var (
	// ErrInvalidWhence is returned when Seek receives a whence value other than
	// io.SeekStart, io.SeekCurrent, or io.SeekEnd.
	ErrInvalidWhence = errors.New("invalid whence")

	// ErrNegativePosition is returned when Seek would move the position before
	// the start of the object.
	ErrNegativePosition = errors.New("negative position")
)

// RangeOpener opens a read of length bytes starting at offset within some
// underlying object. Implementations typically issue a ranged request to a
// remote store (S3 GetObject with a Range header, an HTTP Range request) or
// slice a local resource. The returned body must yield exactly length bytes:
// a shorter body surfaces as io.ErrUnexpectedEOF on the consuming reader, and a
// longer one (a source that ignored the range) as [ErrRangeOverrun]. Bytes past
// the range are never delivered.
type RangeOpener func(ctx context.Context, offset, length int64) (io.ReadCloser, error)

// RangeReadSeeker adapts a [RangeOpener] to io.ReadSeeker for consumers that
// need random access over a remote object without downloading it up front.
// Reads lazily open a ranged request at the current position; Seek only moves
// the position and forces the next Read to reopen, so seek-heavy consumers
// issue one ranged request per repositioning. The caller MUST Close it to
// release the open body, and must not use it from multiple goroutines.
//
// Construct instances with [NewRangeReadSeeker]; the zero value is not usable.
type RangeReadSeeker struct {
	// ctx is captured at construction: io.Reader has no context parameter, and
	// the adapter lives strictly within the request that created it.
	ctx  context.Context
	open RangeOpener
	size int64
	pos  int64
	body io.ReadCloser
}

var _ io.ReadSeekCloser = (*RangeReadSeeker)(nil)

// maxEmptyReads bounds consecutive (0, nil) reads while probing the body end,
// mirroring the limit bufio applies before reporting io.ErrNoProgress.
const maxEmptyReads = 100

// NewRangeReadSeeker returns a lazy io.ReadSeeker over an object served by
// open. size is the known object length; it bounds ranged reads and anchors
// io.SeekEnd. The context is captured for the adapter's lifetime because
// io.Reader carries no context — pass the context of the request the reader
// belongs to.
func NewRangeReadSeeker(ctx context.Context, size int64, open RangeOpener) *RangeReadSeeker {
	return &RangeReadSeeker{ctx: ctx, open: open, size: size}
}

// Read reads from the current position, opening a ranged request on demand.
// It never delivers bytes past the object size. It returns io.EOF once the
// position reaches the object size, io.ErrUnexpectedEOF when the opened body
// ends before the requested range, and [ErrRangeOverrun] (together with the
// final in-range bytes) when the body continues past the range end.
func (r *RangeReadSeeker) Read(p []byte) (int, error) {
	if r.pos >= r.size {
		return 0, io.EOF
	}
	if r.body == nil {
		body, err := r.open(r.ctx, r.pos, r.size-r.pos)
		if err != nil {
			return 0, fmt.Errorf("open ranged read at offset %d: %w", r.pos, err)
		}
		r.body = body
	}
	if remaining := r.size - r.pos; int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := r.body.Read(p)
	r.pos += int64(n)
	if errors.Is(err, io.EOF) {
		r.closeBody()
		if r.pos < r.size {
			return n, io.ErrUnexpectedEOF
		}
		return n, err
	}
	if err == nil && r.pos == r.size {
		err = r.finishRange()
	}
	return n, err
}

// finishRange is called once the whole range has been delivered without the
// body reporting EOF. It probes the body for one more byte, failing with
// [ErrRangeOverrun] if the body continues past the range end, and closes it.
func (r *RangeReadSeeker) finishRange() error {
	defer r.closeBody()
	var probe [1]byte
	for range maxEmptyReads {
		n, err := r.body.Read(probe[:])
		switch {
		case n > 0:
			return fmt.Errorf("%w: body continues past offset %d", ErrRangeOverrun, r.size)
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return fmt.Errorf("read ranged body end at offset %d: %w", r.size, err)
		}
	}
	return io.ErrNoProgress
}

// Seek moves the read position. Seeking away from the current position closes
// the open body so the next Read reopens at the new offset.
func (r *RangeReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var pos int64
	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = r.pos + offset
	case io.SeekEnd:
		pos = r.size + offset
	default:
		return 0, fmt.Errorf("%w: %d", ErrInvalidWhence, whence)
	}
	if pos < 0 {
		return 0, fmt.Errorf("%w: %d", ErrNegativePosition, pos)
	}
	if pos != r.pos {
		r.closeBody()
		r.pos = pos
	}
	return pos, nil
}

// Close releases the open body, if any. It is idempotent.
func (r *RangeReadSeeker) Close() error {
	if r.body == nil {
		return nil
	}
	body := r.body
	r.body = nil
	return body.Close()
}

func (r *RangeReadSeeker) closeBody() {
	if r.body != nil {
		_ = r.body.Close() //nolint:errcheck // best-effort release before reopening at a new offset
		r.body = nil
	}
}

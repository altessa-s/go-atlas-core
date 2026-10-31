// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package io_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	coreio "github.com/altessa-s/go-atlas-core/io"
)

// rangeCall records one RangeOpener invocation.
type rangeCall struct {
	offset, length int64
}

// rangeSource serves a RangeOpener over an in-memory byte slice and records
// every requested range. truncateTo, when positive, caps the returned body
// length to simulate a source returning fewer bytes than the requested range.
// ignoreRange simulates a server that ignores the Range header and returns the
// whole object; trailer appends bytes past the requested range end.
type rangeSource struct {
	mu          sync.Mutex
	data        []byte
	calls       []rangeCall
	openErr     error
	truncateTo  int
	ignoreRange bool
	trailer     []byte
}

func (s *rangeSource) open(_ context.Context, offset, length int64) (io.ReadCloser, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	s.mu.Lock()
	s.calls = append(s.calls, rangeCall{offset: offset, length: length})
	s.mu.Unlock()
	end := min(offset+length, int64(len(s.data)))
	body := s.data[offset:end]
	if s.ignoreRange {
		body = s.data
	}
	if s.truncateTo > 0 && len(body) > s.truncateTo {
		body = body[:s.truncateTo]
	}
	body = append(slices.Clip(body), s.trailer...)
	return io.NopCloser(bytes.NewReader(body)), nil
}

const rangeTestObject = "hello world"

func newTestRangeReadSeeker(t *testing.T, src *rangeSource) *coreio.RangeReadSeeker {
	t.Helper()
	return coreio.NewRangeReadSeeker(t.Context(), int64(len(src.data)), src.open)
}

func TestRangeReadSeeker_SequentialReadWholeObject(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject)}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // released again explicitly below

	got, err := io.ReadAll(rs)
	require.NoError(t, err)
	require.Equal(t, rangeTestObject, string(got))
	require.Equal(t, []rangeCall{{offset: 0, length: int64(len(rangeTestObject))}}, src.calls,
		"a sequential read must open exactly one ranged request from position 0")
	require.NoError(t, rs.Close())
}

func TestRangeReadSeeker_SeekForwardOpensNewRange(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject)}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	head := make([]byte, 5)
	_, err := io.ReadFull(rs, head)
	require.NoError(t, err)
	require.Equal(t, "hello", string(head))

	pos, err := rs.Seek(6, io.SeekStart)
	require.NoError(t, err)
	require.Equal(t, int64(6), pos)

	rest, err := io.ReadAll(rs)
	require.NoError(t, err)
	require.Equal(t, "world", string(rest))

	require.Len(t, src.calls, 2)
	require.Equal(t, int64(6), src.calls[1].offset,
		"the read after a forward seek must open a ranged request at the new position")
}

func TestRangeReadSeeker_SeekWhence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		prepare func(t *testing.T, rs *coreio.RangeReadSeeker)
		offset  int64
		whence  int
		wantPos int64
		want    string
	}{
		{
			name:    "seek end",
			offset:  -5,
			whence:  io.SeekEnd,
			wantPos: 6,
			want:    "world",
		},
		{
			name: "seek current",
			prepare: func(t *testing.T, rs *coreio.RangeReadSeeker) {
				t.Helper()
				_, err := rs.Seek(2, io.SeekStart)
				require.NoError(t, err)
			},
			offset:  4,
			whence:  io.SeekCurrent,
			wantPos: 6,
			want:    "world",
		},
		{
			name:    "seek past end yields EOF on read",
			offset:  2,
			whence:  io.SeekEnd,
			wantPos: int64(len(rangeTestObject)) + 2,
			want:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := &rangeSource{data: []byte(rangeTestObject)}
			rs := newTestRangeReadSeeker(t, src)
			defer rs.Close() //nolint:errcheck // best-effort test cleanup

			if tc.prepare != nil {
				tc.prepare(t, rs)
			}
			pos, err := rs.Seek(tc.offset, tc.whence)
			require.NoError(t, err)
			require.Equal(t, tc.wantPos, pos)

			got, err := io.ReadAll(rs)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))
		})
	}
}

func TestRangeReadSeeker_SeekErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		offset  int64
		whence  int
		wantErr error
	}{
		{name: "negative position", offset: -1, whence: io.SeekStart, wantErr: coreio.ErrNegativePosition},
		{name: "negative position via current", offset: -10, whence: io.SeekCurrent, wantErr: coreio.ErrNegativePosition},
		{name: "invalid whence", offset: 0, whence: 42, wantErr: coreio.ErrInvalidWhence},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := &rangeSource{data: []byte(rangeTestObject)}
			rs := newTestRangeReadSeeker(t, src)
			defer rs.Close() //nolint:errcheck // best-effort test cleanup

			_, err := rs.Seek(tc.offset, tc.whence)
			require.ErrorIs(t, err, tc.wantErr)
			require.Empty(t, src.calls, "a failed seek must not open a ranged read")
		})
	}
}

func TestRangeReadSeeker_ReadAfterEOF(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject)}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	_, err := io.ReadAll(rs)
	require.NoError(t, err)

	n, err := rs.Read(make([]byte, 1))
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF)
}

func TestRangeReadSeeker_ShortBodyIsUnexpectedEOF(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject), truncateTo: 5}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	_, err := io.ReadAll(rs)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF,
		"a body shorter than the requested range must surface io.ErrUnexpectedEOF")
}

func TestRangeReadSeeker_OpenErrorIsWrapped(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("source down")
	src := &rangeSource{data: []byte(rangeTestObject), openErr: sentinel}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	_, err := rs.Read(make([]byte, 1))
	require.ErrorIs(t, err, sentinel)
}

func TestRangeReadSeeker_CloseIsIdempotent(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject)}
	rs := newTestRangeReadSeeker(t, src)

	_, err := rs.Read(make([]byte, 3)) // open a body
	require.NoError(t, err)

	require.NoError(t, rs.Close())
	require.NoError(t, rs.Close(), "Close must be idempotent")
}

func TestRangeReadSeeker_LongerBodyIsRangeOverrun(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		src    *rangeSource
		seekTo int64
		want   string
	}{
		{
			name: "trailing bytes past the range",
			src:  &rangeSource{data: []byte(rangeTestObject), trailer: []byte("EXTRA")},
			want: rangeTestObject,
		},
		{
			name:   "trailing bytes past a range after seek",
			src:    &rangeSource{data: []byte(rangeTestObject), trailer: []byte("!")},
			seekTo: 6,
			want:   "world",
		},
		{
			name:   "server ignored Range at a non-zero offset",
			src:    &rangeSource{data: []byte(rangeTestObject), ignoreRange: true},
			seekTo: 6,
			want:   "hello", // the first bytes of the whole object, capped to the range length
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs := newTestRangeReadSeeker(t, tc.src)
			defer rs.Close() //nolint:errcheck // best-effort test cleanup

			_, err := rs.Seek(tc.seekTo, io.SeekStart)
			require.NoError(t, err)

			got, err := io.ReadAll(rs)
			require.ErrorIs(t, err, coreio.ErrRangeOverrun,
				"a body longer than the requested range must surface ErrRangeOverrun")
			require.Equal(t, tc.want, string(got), "no byte past the range end may be delivered")

			pos, err := rs.Seek(0, io.SeekCurrent)
			require.NoError(t, err)
			require.Equal(t, int64(len(rangeTestObject)), pos, "the position must never pass the object size")

			n, err := rs.Read(make([]byte, 4))
			require.Zero(t, n)
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

func TestRangeReadSeeker_ReadNeverPassesRangeEnd(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject), trailer: []byte("EXTRA")}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	_, err := rs.Seek(-2, io.SeekEnd)
	require.NoError(t, err)

	buf := make([]byte, 64)
	n, err := rs.Read(buf)
	require.Equal(t, 2, n, "a single Read must not deliver more than the remaining range")
	require.Equal(t, "ld", string(buf[:n]))
	require.ErrorIs(t, err, coreio.ErrRangeOverrun)
}

func TestRangeReadSeeker_IgnoredRangeAtOffsetZeroIsExact(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject), ignoreRange: true}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	got, err := io.ReadAll(rs)
	require.NoError(t, err, "a whole-object body for a range starting at 0 is exactly the range")
	require.Equal(t, rangeTestObject, string(got))
}

func TestRangeReadSeeker_ShortBodyAfterSeekIsUnexpectedEOF(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject), truncateTo: 2}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	_, err := rs.Seek(6, io.SeekStart)
	require.NoError(t, err)

	got, err := io.ReadAll(rs)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Equal(t, "wo", string(got))
}

func TestRangeReadSeeker_SeeksAcrossBoundaries(t *testing.T) {
	t.Parallel()

	src := &rangeSource{data: []byte(rangeTestObject)}
	rs := newTestRangeReadSeeker(t, src)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	head := make([]byte, 3)
	_, err := io.ReadFull(rs, head)
	require.NoError(t, err)
	require.Equal(t, "hel", string(head))

	_, err = rs.Seek(1, io.SeekStart)
	require.NoError(t, err)
	mid := make([]byte, 4)
	_, err = io.ReadFull(rs, mid)
	require.NoError(t, err)
	require.Equal(t, "ello", string(mid))

	_, err = rs.Seek(-1, io.SeekEnd)
	require.NoError(t, err)
	buf := make([]byte, 8)
	n, err := rs.Read(buf)
	require.Equal(t, "d", string(buf[:n]))
	if err == nil {
		n, err = rs.Read(buf)
		require.Zero(t, n)
	}
	require.ErrorIs(t, err, io.EOF)

	require.Equal(t, []rangeCall{{0, 11}, {1, 10}, {10, 1}}, src.calls)
}

func TestRangeReadSeeker_ZeroLengthRange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		data   string
		seekTo int64
	}{
		{name: "empty object", data: ""},
		{name: "position at the object end", data: rangeTestObject, seekTo: int64(len(rangeTestObject))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := &rangeSource{data: []byte(tc.data), trailer: []byte("EXTRA")}
			rs := newTestRangeReadSeeker(t, src)
			defer rs.Close() //nolint:errcheck // best-effort test cleanup

			_, err := rs.Seek(tc.seekTo, io.SeekStart)
			require.NoError(t, err)

			n, err := rs.Read(make([]byte, 4))
			require.Zero(t, n)
			require.ErrorIs(t, err, io.EOF)
			require.Empty(t, src.calls, "a zero-length range must not open a ranged read")
		})
	}
}

func TestRangeReadSeeker_BodyErrorAtRangeEndIsSurfaced(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("checksum mismatch")
	open := func(_ context.Context, offset, length int64) (io.ReadCloser, error) {
		body := io.MultiReader(bytes.NewReader([]byte(rangeTestObject)[offset:offset+length]), coreio.NewErrorReader(sentinel))
		return io.NopCloser(body), nil
	}
	rs := coreio.NewRangeReadSeeker(t.Context(), int64(len(rangeTestObject)), open)
	defer rs.Close() //nolint:errcheck // best-effort test cleanup

	got, err := io.ReadAll(rs)
	require.ErrorIs(t, err, sentinel, "an error the body reports at the range end must not be swallowed")
	require.Equal(t, rangeTestObject, string(got))
}

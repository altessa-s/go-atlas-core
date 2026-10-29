// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package wal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
)

// ErrCorruptRecord means a pending record failed its length or checksum check.
var ErrCorruptRecord = errors.New("wal: corrupt pending record")

// ReadPending reads up to limit unacknowledged records in append order. skip
// may exclude records already being delivered; it runs under the WAL lock and
// must not block or call WAL methods. Memory is bounded by the selected records.
// Acknowledgements are process-local; after restart an incompletely acknowledged
// segment may replay already delivered records.
func (w *WAL) ReadPending(limit int, skip func(Offset) bool) ([]Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed.Load() {
		return nil, ErrClosed
	}
	if limit <= 0 {
		return nil, nil
	}
	var records []Record
	for _, seg := range w.sealed {
		if err := w.readPendingSegment(seg, limit, skip, &records); err != nil {
			return nil, err
		}
		if len(records) == limit {
			return records, nil
		}
	}
	if w.active != nil {
		if err := w.readPendingSegment(w.active, limit, skip, &records); err != nil {
			return nil, err
		}
	}
	return records, nil
}

// pendingReadWindow is the span ReadPending fetches per read syscall. Headers
// of skipped and acknowledged records are parsed from memory instead of
// costing one pread each while the WAL lock is held.
const pendingReadWindow = 64 << 10

func (w *WAL) readPendingSegment(seg *segmentRef, limit int, skip func(Offset) bool, out *[]Record) error {
	if seg.unacked.Load() == 0 {
		return nil
	}
	// The group-commit leader publishes size without w.mu; read it once so
	// this pass sees one consistent prefix of fully written records.
	size := seg.size.Load()
	f, err := os.Open(seg.path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // read-only descriptor
	if w.readBuf == nil {
		w.readBuf = make([]byte, pendingReadWindow)
	}
	win := pendingWindow{f: f, buf: w.readBuf}
	for pos := seg.pendingPosition; pos < size && len(*out) < limit; {
		header, err := win.header(pos, size)
		if err != nil {
			return err
		}
		n := int64(binary.LittleEndian.Uint32(header[:4]))
		next := pos + recordHeaderSize + n
		if n == 0 || n > maxRecordBytes || next > size {
			return ErrCorruptRecord
		}
		offset := Offset{SegmentID: seg.id, Position: pos}
		if !seg.skipAcked(pos, next) && (skip == nil || !skip(offset)) {
			payload, err := win.payload(pos, n)
			if err != nil {
				return err
			}
			if crc32.ChecksumIEEE(payload) != binary.LittleEndian.Uint32(header[4:]) {
				return ErrCorruptRecord
			}
			*out = append(*out, Record{Offset: offset, Payload: payload})
		}
		pos = next
	}
	return nil
}

// pendingWindow buffers a span of a segment file so record headers (and
// small payloads) are parsed from memory rather than one pread each.
type pendingWindow struct {
	f     *os.File
	buf   []byte
	data  []byte // bytes of the segment starting at start
	start int64
}

// header returns the record header at pos, refilling the window from pos
// when the header is not fully inside it.
func (pw *pendingWindow) header(pos, size int64) ([]byte, error) {
	rel := pos - pw.start
	if pw.data == nil || rel+recordHeaderSize > int64(len(pw.data)) {
		n := min(int64(len(pw.buf)), size-pos)
		if _, err := pw.f.ReadAt(pw.buf[:n], pos); err != nil {
			return nil, err
		}
		pw.data, pw.start, rel = pw.buf[:n], pos, 0
	}
	return pw.data[rel : rel+recordHeaderSize], nil
}

// payload returns a copy of the n-byte payload of the record at pos, from
// the window when it holds it, from the file otherwise.
func (pw *pendingWindow) payload(pos, n int64) ([]byte, error) {
	payload := make([]byte, n)
	if start := pos - pw.start + recordHeaderSize; start+n <= int64(len(pw.data)) {
		copy(payload, pw.data[start:start+n])
		return payload, nil
	}
	if _, err := pw.f.ReadAt(payload, pos+recordHeaderSize); err != nil {
		return nil, err
	}
	return payload, nil
}

// skipAcked reports whether the record at pos was acknowledged. An
// acknowledged record at the pending position advances it to next.
func (s *segmentRef) skipAcked(pos, next int64) bool {
	_, acked := s.acked[pos]
	if acked && pos == s.pendingPosition {
		delete(s.acked, pos)
		s.pendingPosition = next
	}
	return acked
}

func (s *segmentRef) ack(position int64) {
	if position < s.pendingPosition || position < 0 || position >= s.size.Load() || s.unacked.Load() == 0 {
		return
	}
	if _, ok := s.acked[position]; ok {
		return
	}
	if s.acked == nil {
		s.acked = make(map[int64]struct{})
	}
	s.acked[position] = struct{}{}
	s.unacked.Add(-1)
}

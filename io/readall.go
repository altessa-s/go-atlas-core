// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package io

import (
	"errors"
	"io"
	"math"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// ReadAllLimited reads r until EOF, like [io.ReadAll], but accepts at most n
// bytes. It reads at most n+1 bytes from r: when more than n bytes are
// available it returns the first n bytes and an error wrapping
// [ErrReadLimitExceeded] instead of silently truncating — also when the excess
// byte arrives together with a read error, which is then joined to it. Any
// other read error is returned unchanged together with the bytes read before
// it, as [io.ReadAll] does. A negative n is treated as 0. r is not closed.
func ReadAllLimited(r io.Reader, n int64) ([]byte, error) {
	n = max(n, 0)
	if n == math.MaxInt64 {
		// n+1 would overflow; no reader can exceed this limit.
		return io.ReadAll(r)
	}
	data, err := io.ReadAll(io.LimitReader(r, n+1))
	if int64(len(data)) > n {
		// A reader may hand over the excess byte together with an error; the
		// overflow is still reported, joined with that error.
		return data[:n], errors.Join(coreerrs.Wrapf(ErrReadLimitExceeded, "more than %d bytes available", n), err)
	}
	return data, err
}

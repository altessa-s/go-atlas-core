// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build unix

package files

import (
	"os"
	"syscall"
)

// openRegularFlags opens without blocking, so a FIFO without a writer does
// not hang the open.
const openRegularFlags = os.O_RDONLY | syscall.O_NONBLOCK

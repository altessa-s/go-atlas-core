// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

//go:build !unix

package files

import "os"

// openRegularFlags is a plain read-only open: the platform has no FIFOs that
// block opening.
const openRegularFlags = os.O_RDONLY

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package intern

//go:generate optgen generate

import "github.com/altessa-s/go-atlas-core/time/clock"

// defaultClock is the time source an [Interner] uses when [WithClock] is not
// supplied.
var defaultClock = clock.Real()

// options holds the [Interner] configuration. Configure it through the
// generated [Option] values (the With* constructors).
type options struct {
	// clock supplies the access timestamps used to break eviction ties by
	// least recent access.
	clock clock.Clock `optgen:"default=defaultClock,notnil"`
}

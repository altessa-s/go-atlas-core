// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package retry

//go:generate optgen generate --type=exponentialOptions --output=exponential_options_gen.go --option-type=ExponentialOption

import (
	"time"
)

// DefaultExponentialFactor is the exponential multiplier [Exponential] applies
// per attempt unless [WithExponentialFactor] sets another.
const DefaultExponentialFactor = 1.5

// exponentialOptions configures [Exponential].
type exponentialOptions struct {
	// maxDelay caps the computed delay. A zero or negative value leaves the
	// delay unbounded.
	maxDelay time.Duration `opt:"ExponentialMaxDelay"`
	// factor is the multiplier applied per attempt
	// (delay = baseDelay * factor^attempt). Zero means [DefaultExponentialFactor].
	factor float64 `opt:"ExponentialFactor" optgen:"default=DefaultExponentialFactor"`
	// jitter is the maximum fraction by which jitter varies the computed
	// delay (0.0–1.0); values above 1.0 are clamped to 1.0. Zero (the
	// default) gives deterministic delays.
	jitter float64 `opt:"ExponentialJitter"`
	// WithExponentialJitterMode selects how jitter varies the capped delay.
	// Unknown modes use [JitterAdditive].
	jitterMode JitterMode `opt:"ExponentialJitterMode" optgen:"default=JitterAdditive"`
	// jitterSource draws the jitter samples. Nil means math/rand/v2; a sample outside [0, 1) is clamped into it.
	jitterSource JitterSource `opt:"ExponentialJitterSource"`
}

// JitterSource returns samples uniform in [0, 1), the u of each [JitterMode] formula. A source shared between
// goroutines must be safe for concurrent use.
type JitterSource func() float64

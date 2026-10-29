// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
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
	// jitter is the maximum fraction of the computed delay added as random
	// jitter (0.0–1.0); values above 1.0 are clamped to 1.0. Zero (the
	// default) gives deterministic delays.
	jitter float64 `opt:"ExponentialJitter"`
}

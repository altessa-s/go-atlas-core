// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package singleflight

//go:generate optgen generate

import (
	"context"
	"time"
)

// DefaultMaxConcurrent limits distinct computations admitted at once.
const DefaultMaxConcurrent = 1024

// DefaultTimeout bounds the context passed to a shared computation.
const DefaultTimeout = 15 * time.Second

type options struct {
	// lifecycle cancels work independently of callers; nil preserves legacy behavior.
	lifecycle context.Context
	// panicAsError replaces recovered panics without retaining their values.
	panicAsError error

	maxConcurrent int           `optgen:"default=DefaultMaxConcurrent"`
	timeout       time.Duration `optgen:"default=DefaultTimeout"`
}

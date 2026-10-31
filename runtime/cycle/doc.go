// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package cycle guards a function driven manually or by an external owner.
// Entry and MarkManaged separate registration from ownership transfer. TryRun
// and TryRunFunc let internal drivers share the same guard regardless of ownership.
// Overlapping calls skip execution and return nil. Panics propagate to the
// caller after releasing the guard; callers can defer runtime/panics.Handle
// at their execution boundary to apply the application's panic policy.
package cycle

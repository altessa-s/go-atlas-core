// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package capabilities_test

import (
	"errors"
	"log"
	"runtime"

	"github.com/altessa-s/go-atlas-core/runtime/capabilities"
)

// ExampleDropAll demonstrates the "nuclear option" — drop every
// capability before loading untrusted code. Compile-checked but not
// executed, because dropping capabilities is irreversible and would
// poison every subsequent test in the binary.
func ExampleDropAll() {
	// The drop applies to the calling OS thread only: keep the
	// restricted work on this thread. A process-wide drop must happen
	// before the binary starts (systemd, container --cap-drop, launcher).
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := capabilities.DropAll()
	switch {
	case err == nil:
		// This thread now has zero capabilities; other runtime threads
		// keep theirs.
	case errors.Is(err, capabilities.ErrUnsupported):
		log.Print("capabilities: platform does not support Linux capabilities; continuing")
	case errors.Is(err, capabilities.ErrFailed):
		log.Fatalf("capabilities: kernel rejected capset: %v", err)
	default:
		log.Fatalf("capabilities: unexpected error: %v", err)
	}
}

// ExampleDropAllExcept demonstrates the "bind a privileged port, then
// drop everything else" pattern — the most common real-world use
// case for capability management.
func ExampleDropAllExcept() {
	// Bind the socket first, while CAP_NET_BIND_SERVICE is still
	// effective. (Elided here because this is a compile-checked
	// example, not a runnable one.)
	//
	// The change applies to the calling OS thread only: keep the
	// restricted work on this thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := capabilities.DropAllExcept(capabilities.CAP_NET_BIND_SERVICE)
	switch {
	case err == nil:
		// On this thread only CAP_NET_BIND_SERVICE remains in the
		// effective and permitted sets; every other capability is
		// dropped from its bounding set too, so this thread and its
		// descendants cannot re-acquire it.
	case errors.Is(err, capabilities.ErrUnsupported):
		log.Print("capabilities: platform does not support Linux capabilities; continuing")
	case errors.Is(err, capabilities.ErrInvalidOption):
		log.Fatalf("capabilities: bad config: %v", err)
	case errors.Is(err, capabilities.ErrFailed):
		log.Fatalf("capabilities: kernel rejected capset: %v", err)
	default:
		log.Fatalf("capabilities: unexpected error: %v", err)
	}
}

// ExampleParseName shows how to resolve operator-supplied capability
// names (case-insensitive, whitespace-tolerant) from YAML config.
func ExampleParseName() {
	names := []string{"CAP_NET_BIND_SERVICE", "cap_net_raw"}
	keep := make([]capabilities.Cap, 0, len(names))
	for _, name := range names {
		c, err := capabilities.ParseName(name)
		if err != nil {
			log.Fatalf("capabilities: %v", err)
		}
		keep = append(keep, c)
	}
	_ = keep // ... pass to DropAllExcept or Apply.
}

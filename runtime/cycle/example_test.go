// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package cycle_test

import (
	"context"
	"fmt"

	"github.com/altessa-s/go-atlas-core/runtime/cycle"
)

func ExampleGuard() {
	g := cycle.New(func(context.Context) error { fmt.Println("cycle"); return nil })
	_ = g.Run(context.Background())
	drive := g.Managed()
	fmt.Println(g.Run(context.Background()))
	_ = drive(context.Background())
	// Output:
	// cycle
	// cycle: managed, direct calls not allowed
	// cycle
}

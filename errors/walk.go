// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package errors

import "iter"

// Walk visits err and its Unwrap children in pre-order, depth first and left to
// right, like errors.Is. A false visit result stops before unwrapping that node.
// Walk returns true on completion or a requested stop, and false only when a
// further visit would exceed budget. A nil root always completes. A non-nil
// root with budget <= 0 receives no visits and returns false.
//
// Shared nodes count once per path. Cycles consume the budget. Nil slots in
// child slices consume a visit but are never passed to visit. A nil result
// from Unwrap() error has no children.
func Walk(err error, budget int, visit func(error) bool) bool {
	if err == nil {
		return true
	}

	// Keep only the unvisited suffix of each join. A wide join takes one
	// stack entry regardless of how many children it returns.
	var pending [][]error

	for {
		if budget <= 0 {
			return false
		}

		budget--

		if err != nil {
			if !visit(err) {
				return true
			}

			switch u := err.(type) {
			case interface{ Unwrap() error }:
				if err = u.Unwrap(); err != nil {
					continue
				}
			case interface{ Unwrap() []error }:
				if group := u.Unwrap(); len(group) != 0 {
					pending = append(pending, group)
				}
			}
		}

		if len(pending) == 0 {
			return true
		}

		last := len(pending) - 1
		group := pending[last]

		err = group[0]
		if len(group) == 1 {
			pending[last] = nil
			pending = pending[:last]
		} else {
			pending[last] = group[1:]
		}
	}
}

// WalkFunc behaves like [Walk] with caller-defined children in traversal order.
// Yield no children to prune a subtree, or yield private causes after ordinary
// children to inspect both with one budget. The sequence must honor early stop.
// A nil children function selects the standard Unwrap methods.
func WalkFunc(err error, budget int, visit func(error) bool, children func(error) iter.Seq[error]) bool {
	if children == nil {
		return Walk(err, budget, visit)
	}

	if err == nil {
		return true
	}

	stopped := false

	var walk func(error) bool

	walk = func(node error) bool {
		if budget <= 0 {
			return false
		}

		budget--

		if node == nil {
			return true
		}

		if !visit(node) {
			stopped = true
			return false
		}

		for child := range children(node) {
			if !walk(child) {
				return false
			}
		}

		return true
	}

	return walk(err) || stopped
}

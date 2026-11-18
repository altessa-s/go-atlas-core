// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package errors_test

import (
	"errors"
	"fmt"
	"iter"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	coreerrors "github.com/altessa-s/go-atlas-core/errors"
)

type loopError struct{}

func (*loopError) Error() string   { return "loop" }
func (e *loopError) Unwrap() error { return e }

type joinError []error

func (joinError) Error() string     { return "join" }
func (e joinError) Unwrap() []error { return e }

func TestWalk(t *testing.T) {
	t.Parallel()

	leaf := errors.New("leaf")
	branch := fmt.Errorf("branch: %w", leaf)
	tree := errors.Join(branch, leaf)
	cycle := &loopError{}
	joinedCycle := joinError{leaf, nil}
	joinedCycle[1] = joinedCycle

	wide := make(joinError, 4096)
	for i := range wide {
		wide[i] = leaf
	}

	for _, tc := range []struct {
		name           string
		root           error
		budget, stopAt int
		want           []error
		complete       bool
	}{
		{"nil", nil, -1, 0, nil, true},
		{"zero", leaf, 0, 0, nil, false},
		{"negative", leaf, -1, 0, nil, false},
		{"exact", tree, 4, 0, []error{tree, branch, leaf, leaf}, true},
		{"short", tree, 3, 0, []error{tree, branch, leaf}, false},
		{"stop", tree, 2, 2, []error{tree, branch}, true},
		{"cycle", cycle, 3, 0, []error{cycle, cycle, cycle}, false},
		{"join cycle", joinedCycle, 3, 0, []error{joinedCycle, leaf, joinedCycle}, false},
		{"wide", wide, 2, 0, []error{wide, leaf}, false},
		{"nil slots", joinError{nil, leaf}, 2, 0, []error{joinError{nil, leaf}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var visited []error

			complete := coreerrors.Walk(tc.root, tc.budget, func(node error) bool {
				visited = append(visited, node)
				return len(visited) != tc.stopAt
			})
			require.Equal(t, tc.complete, complete)
			require.Equal(t, tc.want, visited)
		})
	}
}

func TestWalkFunc(t *testing.T) {
	t.Parallel()

	leaf := errors.New("masked")
	pruned := fmt.Errorf("pruned: %w", leaf)
	hidden := errors.New("hidden")
	root := errors.Join(pruned)

	children := func(node error) iter.Seq[error] {
		if node == root {
			return slices.Values([]error{pruned, hidden})
		}

		return slices.Values([]error(nil))
	}

	for _, tc := range []struct {
		name     string
		budget   int
		stop     error
		want     []error
		complete bool
	}{
		{"custom edges", 3, nil, []error{root, pruned, hidden}, true},
		{"shared budget", 2, nil, []error{root, pruned}, false},
		{"early stop", 2, pruned, []error{root, pruned}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var visited []error

			ok := coreerrors.WalkFunc(root, tc.budget, func(node error) bool {
				visited = append(visited, node)
				return node != tc.stop
			}, children)
			require.Equal(t, tc.complete, ok)
			require.Equal(t, tc.want, visited)
		})
	}
}

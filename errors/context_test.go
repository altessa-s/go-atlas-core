// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package errors_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/errors"

	std_errors "errors"
)

func TestContextClassifiers(t *testing.T) {
	t.Parallel()

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name             string
		err              error
		canceled         bool
		deadlineExceeded bool
	}{
		{name: "nil", err: nil},
		{name: "unrelated", err: std_errors.New("other")},
		{name: "canceled", err: context.Canceled, canceled: true},
		{name: "wrapped canceled", err: fmt.Errorf("query: %w", context.Canceled), canceled: true},
		{name: "context cause", err: canceledCtx.Err(), canceled: true},
		{name: "deadline", err: context.DeadlineExceeded, deadlineExceeded: true},
		{name: "wrapped deadline", err: errors.Wrap(context.DeadlineExceeded, "fetch"), deadlineExceeded: true},
		{
			name:             "joined",
			err:              std_errors.Join(context.Canceled, context.DeadlineExceeded),
			canceled:         true,
			deadlineExceeded: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.canceled, errors.IsContextCanceled(tc.err))
			require.Equal(t, tc.deadlineExceeded, errors.IsContextDeadlineExceeded(tc.err))
			require.Equal(t, tc.canceled || tc.deadlineExceeded, errors.IsContextCanceledOrDeadlineExceeded(tc.err))
		})
	}
}

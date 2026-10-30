// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package errors_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// FuzzContextClassifiersAgreeWithErrorsIs pins the context helpers against the
// standard library they wrap.
//
// These decide whether a failed dispatch is transient — the outbox uses exactly
// this to avoid dead-lettering healthy events during a broker outage — so a
// divergence from errors.Is is a classification the rest of the codebase
// assumes cannot happen.
func FuzzContextClassifiersAgreeWithErrorsIs(f *testing.F) {
	f.Add(uint8(0), "wrapped", uint8(2))
	f.Add(uint8(1), "", uint8(0))
	f.Add(uint8(2), "op", uint8(5))

	f.Fuzz(func(t *testing.T, kind uint8, message string, depth uint8) {
		bases := []error{context.Canceled, context.DeadlineExceeded, errors.New(message), nil}
		base := bases[int(kind)%len(bases)]

		err := base
		for range int(depth) % 8 {
			if err == nil {
				break
			}
			err = fmt.Errorf("%s: %w", message, err)
		}

		require.Equal(t, err != nil && errors.Is(err, context.Canceled), coreerrs.IsContextCanceled(err))
		require.Equal(t, err != nil && errors.Is(err, context.DeadlineExceeded), coreerrs.IsContextDeadlineExceeded(err))
		require.Equal(t,
			coreerrs.IsContextCanceled(err) || coreerrs.IsContextDeadlineExceeded(err),
			coreerrs.IsContextCanceledOrDeadlineExceeded(err),
			"the combined classifier is not the disjunction of its parts")
	})
}

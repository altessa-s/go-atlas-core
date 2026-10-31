// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package slices_test

import (
	"container/list"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	coreslices "github.com/altessa-s/go-atlas-core/collections/slices"
)

type namedInts []int

func TestTo(t *testing.T) {
	t.Parallel()

	t.Run("transforms in order", func(t *testing.T) {
		t.Parallel()
		got := coreslices.To([]int{3, 1, 2}, strconv.Itoa)
		require.Equal(t, []string{"3", "1", "2"}, got)
	})

	t.Run("accepts named slice types", func(t *testing.T) {
		t.Parallel()
		got := coreslices.To(namedInts{1, 2}, func(n int) int { return n * 10 })
		require.Equal(t, []int{10, 20}, got)
	})

	t.Run("empty and nil return nil without calling fn", func(t *testing.T) {
		t.Parallel()
		fn := func(int) string {
			t.Fatal("fn must not be called for an empty collection")
			return ""
		}
		require.Nil(t, coreslices.To([]int{}, fn))
		require.Nil(t, coreslices.To([]int(nil), fn))
	})
}

func TestToStrings_Edges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []any
		want  []string
	}{
		{name: "nil", input: nil, want: nil},
		{name: "empty", input: []any{}, want: nil},
		{name: "no strings", input: []any{1, true, 2.5, nil}, want: nil},
		{name: "empty string kept", input: []any{"", 1, "x"}, want: []string{"", "x"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, coreslices.ToStrings(tc.input))
		})
	}
}

func TestToWithFilter_Edges(t *testing.T) {
	t.Parallel()

	isEven := func(n int) bool { return n%2 == 0 }

	t.Run("empty returns nil", func(t *testing.T) {
		t.Parallel()
		got := coreslices.ToWithFilter([]int(nil), isEven, strconv.Itoa)
		require.Nil(t, got)
	})

	t.Run("nothing matches returns nil", func(t *testing.T) {
		t.Parallel()
		got := coreslices.ToWithFilter([]int{1, 3, 5}, isEven, strconv.Itoa)
		require.Nil(t, got)
	})

	t.Run("fn runs only for matching elements", func(t *testing.T) {
		t.Parallel()
		var calls []int
		got := coreslices.ToWithFilter([]int{1, 2, 3, 4}, isEven, func(n int) string {
			calls = append(calls, n)
			return strconv.Itoa(n)
		})
		require.Equal(t, []string{"2", "4"}, got)
		require.Equal(t, []int{2, 4}, calls)
	})
}

func TestAppendNonNilErr(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	one := 1

	tests := []struct {
		name    string
		value   *int
		err     error
		wantLen int
		wantErr error
	}{
		{name: "non-nil value is appended", value: &one, wantLen: 2},
		{name: "nil value is skipped", value: nil, wantLen: 1},
		{name: "error leaves slice unchanged", value: &one, err: errBoom, wantLen: 1, wantErr: errBoom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start := []*int{new(0)}
			got, err := coreslices.AppendNonNilErr(start, func() (*int, error) { return tc.value, tc.err })
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, got, tc.wantLen)
			require.Same(t, start[0], got[0])
			if tc.wantLen == 2 {
				require.Same(t, tc.value, got[1])
			}
		})
	}
}

func TestList_EarlyBreakAndTypeFilter(t *testing.T) {
	t.Parallel()

	l := list.New()
	l.PushBack(1)
	l.PushBack("skip")
	l.PushBack(2)
	l.PushBack(3)

	var got []int
	for v := range coreslices.List[int](l) {
		got = append(got, v)
		if v == 2 {
			break
		}
	}
	require.Equal(t, []int{1, 2}, got)

	require.Empty(t, slices.Collect(coreslices.List[int](nil)))
}

func TestMap_EarlyBreakStopsTransforming(t *testing.T) {
	t.Parallel()

	var calls int
	seq := coreslices.Map([]int{1, 2, 3, 4}, func(n int) int {
		calls++
		return n * 2
	})
	var got []int
	for v := range seq {
		got = append(got, v)
		if len(got) == 2 {
			break
		}
	}
	require.Equal(t, []int{2, 4}, got)
	require.Equal(t, 2, calls)
}

func TestMapSeq_EarlyBreakStopsSource(t *testing.T) {
	t.Parallel()

	var pulled int
	src := func(yield func(int) bool) {
		for i := range 10 {
			pulled++
			if !yield(i) {
				return
			}
		}
	}
	var got []string
	for s := range coreslices.MapSeq(src, strconv.Itoa) {
		got = append(got, s)
		if len(got) == 3 {
			break
		}
	}
	require.Equal(t, []string{"0", "1", "2"}, got)
	require.Equal(t, 3, pulled)
}

func TestPool_ZeroValueGet(t *testing.T) {
	t.Parallel()

	var pool coreslices.Pool[int]
	buf := pool.Get()
	require.NotNil(t, buf)
	require.Empty(t, *buf)

	*buf = append(*buf, 1, 2, 3)
	pool.Put(buf)

	again := pool.Get()
	require.NotNil(t, again)
	require.Empty(t, *again)
}

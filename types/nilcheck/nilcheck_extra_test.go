// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package nilcheck_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/types/nilcheck"
)

func TestRequiredError_Message(t *testing.T) {
	t.Parallel()

	err := nilcheck.RequireNotNil((*int)(nil), "logger")
	require.Error(t, err)
	require.EqualError(t, err, "logger is required")

	reqErr, ok := errors.AsType[*nilcheck.RequiredError](err)
	require.True(t, ok)
	require.Equal(t, "logger", reqErr.FieldName)
}

func TestIsNil_NillableKinds(t *testing.T) {
	t.Parallel()

	var (
		nilMap   map[string]int
		nilSlice []int
		nilChan  chan int
		nilFunc  func()
		typedNil error = (*nilcheck.RequiredError)(nil)
	)
	tests := []struct {
		name  string
		input any
		want  bool
	}{
		{name: "nil map", input: nilMap, want: true},
		{name: "empty map", input: map[string]int{}, want: false},
		{name: "nil slice", input: nilSlice, want: true},
		{name: "empty slice", input: []int{}, want: false},
		{name: "nil chan", input: nilChan, want: true},
		{name: "open chan", input: make(chan int), want: false},
		{name: "nil func", input: nilFunc, want: true},
		{name: "func", input: func() {}, want: false},
		{name: "typed nil in interface", input: typedNil, want: true},
		{name: "struct", input: struct{}{}, want: false},
		{name: "zero string", input: "", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, nilcheck.IsNil(tc.input))
		})
	}
}

func TestIsNotNilValue(t *testing.T) {
	t.Parallel()

	var (
		nilPtr   *int
		nilIface error
	)
	tests := []struct {
		name  string
		input reflect.Value
		want  bool
	}{
		{name: "invalid value", input: reflect.Value{}, want: false},
		{name: "nil pointer", input: reflect.ValueOf(nilPtr), want: false},
		{name: "nil interface field", input: reflect.ValueOf(&nilIface).Elem(), want: false},
		{name: "non-nil pointer", input: reflect.ValueOf(new(1)), want: true},
		{name: "int", input: reflect.ValueOf(0), want: true},
		{name: "empty slice", input: reflect.ValueOf([]int{}), want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, nilcheck.IsNotNilValue(tc.input))
			require.Equal(t, !tc.want, nilcheck.IsNilValue(tc.input))
		})
	}
}

func TestIsEmptyValue_StringPointers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: true},
		{name: "ascii whitespace", value: " \t\r\n", want: true},
		{name: "unicode whitespace", value: "  ", want: true},
		{name: "padded text", value: "  a  ", want: false},
		{name: "text", value: "x", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := tc.value
			require.Equal(t, tc.want, nilcheck.IsEmptyValue(reflect.ValueOf(&s)))
		})
	}

	require.True(t, nilcheck.IsEmptyValue(reflect.Value{}), "invalid value is empty")
	require.True(t, nilcheck.IsEmptyValue(reflect.ValueOf((*string)(nil))), "nil *string is empty")
	require.False(t, nilcheck.IsEmptyValue(reflect.ValueOf(new(0))), "non-nil *int is not empty")
}

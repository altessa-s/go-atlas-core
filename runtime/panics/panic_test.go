// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package panics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetGlobalPanicHandlers_StoresCopy(t *testing.T) {
	h := func(context.Context, any) {}

	SetGlobalPanicHandlers(h)
	gotAny := globalPanicHandlers.Load()
	got, ok := gotAny.(PanicHandlers)
	require.True(t, ok, "unexpected type in globalPanicHandlers: %T", gotAny)
	require.Len(t, got, 1)

	// Mutate the original slice passed to SetGlobalPanicHandlers (best-effort).
	handlers := []PanicHandler{h}
	SetGlobalPanicHandlers(handlers...)
	handlers[0] = nil

	gotAny2 := globalPanicHandlers.Load()
	got2 := gotAny2.(PanicHandlers)
	require.NotNil(t, got2[0], "stored handlers were mutated via external slice; expected copy")
}

func TestAddGlobalPanicHandler_NilIsIgnored(t *testing.T) {
	// Ensure no panic and no change to state beyond what's already there.
	before := globalPanicHandlers.Load().(PanicHandlers)
	AddGlobalPanicHandler(nil)
	after := globalPanicHandlers.Load().(PanicHandlers)
	require.Len(t, after, len(before))
}

func TestMustNonNil(t *testing.T) {
	// Non-nil should not panic
	MustNonNil("value", "should not panic")

	// Nil should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil")
		}
	}()
	MustNonNil(nil, "nil value")
}

func TestMustNonZero(t *testing.T) {
	MustNonZero(42, "should not panic")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero")
		}
	}()
	MustNonZero(0, "zero value")
}

// TestMustNonZero_NilAny exercises the v == nil branch, distinct from the
// reflect.IsZero branch exercised by TestMustNonZero's zero-int case: a
// literal nil passed as the `any` parameter short-circuits before
// reflect.ValueOf is ever called.
func TestMustNonZero_NilAny(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil")
		}
	}()
	MustNonZero(nil, "nil value")
}

// namedString is a defined type whose underlying type is string, used to
// exercise panicMsg's default case: the type switch on msg matches the
// literal dynamic types "string" and "*string", not types with a
// different underlying-but-compatible type.
type namedString string

// TestMust_NamedStringType_HitsDefaultCase exercises panicMsg's default
// branch, unreachable via the plain string/*string cases already covered
// by other tests.
func TestMust_NamedStringType_HitsDefaultCase(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "expected panic")
		s, ok := r.(string)
		require.True(t, ok, "expected string, got %T", r)
		require.Contains(t, s, "boom")
	}()
	Must(false, namedString("boom"))
}

// TestPanicLoggerHandler_StringPanicUsesContextLogger exercises the default
// panic handler's string-panic branch and the context-logger priority path.
func TestPanicLoggerHandler_StringPanicUsesContextLogger(t *testing.T) {
	savedLogger := panicLogger.Load()
	savedCtxFn := loggerFromContext.Load()
	t.Cleanup(func() {
		if savedLogger != nil {
			panicLogger.Store(savedLogger)
		}
		if savedCtxFn != nil {
			loggerFromContext.Store(savedCtxFn)
		}
	})

	var ctxBuf, globalBuf bytes.Buffer
	ctxLogger := slog.New(slog.NewTextHandler(&ctxBuf, nil))
	panicLogger.Store(slog.New(slog.NewTextHandler(&globalBuf, nil)))
	loggerFromContext.Store(LoggerFromContextFunc(func(context.Context) *slog.Logger { return ctxLogger }))

	panicLoggerHandler(t.Context(), "boom from ctx")

	require.Contains(t, ctxBuf.String(), "boom from ctx")
	require.Contains(t, ctxBuf.String(), "observed a panic")
	require.Empty(t, globalBuf.String(), "global logger must not be used when a context logger is available")
}

// TestPanicLoggerHandler_NonStringPanicUsesGlobalLogger exercises the
// non-string panic-value branch and the global-logger fallback path (no
// context logger registered).
func TestPanicLoggerHandler_NonStringPanicUsesGlobalLogger(t *testing.T) {
	savedLogger := panicLogger.Load()
	savedCtxFn := loggerFromContext.Load()
	t.Cleanup(func() {
		if savedLogger != nil {
			panicLogger.Store(savedLogger)
		}
		if savedCtxFn != nil {
			loggerFromContext.Store(savedCtxFn)
		}
	})

	var globalBuf bytes.Buffer
	panicLogger.Store(slog.New(slog.NewTextHandler(&globalBuf, nil)))
	loggerFromContext.Store(LoggerFromContextFunc(func(context.Context) *slog.Logger { return nil }))

	panicLoggerHandler(t.Context(), errors.New("boom error"))

	require.Contains(t, globalBuf.String(), "boom error")
}

func TestMustError(t *testing.T) {
	// nil error should not panic
	MustError(nil)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-nil error")
		}
	}()
	MustError(fmt.Errorf("bad"))
}

func TestMustResult(t *testing.T) {
	got := MustResult(42, nil)
	require.Equal(t, 42, got)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	MustResult(0, fmt.Errorf("error"))
}

func TestMust(t *testing.T) {
	Must(true, "should not panic")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for false")
		}
	}()
	Must(false, "condition failed")
}

func TestInvalidArgument(t *testing.T) {
	// ok=false should NOT panic
	InvalidArgument(false, "fine")

	defer func() {
		r := recover()
		require.NotNil(t, r, "expected panic")
		err, ok := r.(error)
		require.True(t, ok, "expected error, got %T", r)
		require.ErrorIs(t, err, ErrInvalidArgument)
	}()
	InvalidArgument(true, "bad arg")
}

func TestHandle(t *testing.T) {
	var captured any
	func() {
		defer Handle(t.Context(), func(_ context.Context, r any) {
			captured = r
		})
		panic("test panic")
	}()
	require.Equal(t, "test panic", captured)
}

func TestHandleWithOpts(t *testing.T) {
	var captured any
	func() {
		opts := NewHandleOpts().SetReallyPanic(false)
		defer HandleWithOpts(t.Context(), opts, func(_ context.Context, r any) {
			captured = r
		})
		panic("opts panic")
	}()
	require.Equal(t, "opts panic", captured)
}

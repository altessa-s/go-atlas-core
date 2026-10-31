// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package singleflight

import (
	"context"
	"errors"
	"math"
	"testing"
	"testing/synctest"
)

func TestNonReflexiveKeys(t *testing.T) {
	t.Parallel()
	t.Run("NaN", func(t *testing.T) {
		testNonReflexiveKey(t, math.NaN())
	})
	t.Run("struct", func(t *testing.T) {
		testNonReflexiveKey(t, struct{ Value float64 }{Value: math.NaN()})
	})
}

func testNonReflexiveKey[K comparable](t *testing.T, key K) {
	t.Helper()
	t.Run("retention", func(t *testing.T) {
		g := New[K, int](WithMaxConcurrent(1))
		defer g.Wait()
		for i := range 1000 {
			got, err := g.Do(t.Context(), key, func(context.Context) (int, error) { return i, nil })
			if err != nil || got != i {
				t.Fatalf("Do() = (%d, %v), want (%d, nil)", got, err, i)
			}
			g.mu.Lock()
			retained, active := len(g.calls), g.active
			g.mu.Unlock()
			if retained != 0 || active != 0 {
				t.Fatalf("after call %d: retained = %d, active = %d; want zero", i, retained, active)
			}
		}
	})
	t.Run("capacity_and_wait", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			g := New[K, int](WithMaxConcurrent(2))
			release := make(chan struct{})
			defer close(release)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			started := make(chan struct{}, 2)
			for range 2 {
				go func() {
					_, shared, err := g.DoShared(ctx, key, func(context.Context) (int, error) {
						started <- struct{}{}
						<-release
						return 1, nil
					})
					if !errors.Is(err, context.Canceled) || shared {
						t.Errorf("DoShared() shared = %v, err = %v; want false, context.Canceled", shared, err)
					}
				}()
			}
			synctest.Wait()
			if len(started) != 2 {
				t.Fatalf("started %d callbacks, want 2 independent callbacks", len(started))
			}
			cancel()
			synctest.Wait()
			g.Forget(key)
			_, err := g.Do(t.Context(), key, func(context.Context) (int, error) {
				t.Error("overloaded work started")
				return 0, nil
			})
			if !errors.Is(err, ErrOverloaded) {
				t.Fatalf("Do() error = %v, want ErrOverloaded", err)
			}
			waited := make(chan struct{})
			go func() {
				g.Wait()
				close(waited)
			}()
			synctest.Wait()
			select {
			case <-waited:
				t.Fatal("Wait returned while callbacks were blocked")
			default:
			}
			// Release both callbacks and verify Wait observes their completion.
			release <- struct{}{}
			release <- struct{}{}
			<-waited
			if len(g.calls) != 0 || g.active != 0 {
				t.Fatalf("after Wait: retained = %d, active = %d; want zero", len(g.calls), g.active)
			}
		})
	})
}

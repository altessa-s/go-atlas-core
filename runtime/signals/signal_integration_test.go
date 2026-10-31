//go:build integration && unix

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Integration tests that deliver real OS signals with kill(2). Every
// scenario runs in a re-executed copy of the test binary so a signal
// the dispatcher stops handling terminates the child, never the test
// process. The child reports what its handlers observed as one line
// per event on stdout; the parent drives it with kill(2) and asserts
// on the lines in order.

package signals_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altessa-s/go-atlas-core/runtime/signals"
)

// signalScenarioEnv selects the scenario the re-executed child runs.
const signalScenarioEnv = "GO_ATLAS_SIGNALS_ITEST_SCENARIO"

// lineTimeout bounds every wait for a child event. It is generous on
// purpose: the events arrive within milliseconds on an idle machine and
// the bound only exists so a broken dispatcher fails the test instead of
// hanging it.
const lineTimeout = 10 * time.Second

// signalChild is the parent's handle on a re-executed scenario child.
type signalChild struct {
	cmd   *exec.Cmd
	lines chan string
}

// startSignalChild re-executes the test binary into TestSignalChild with
// the given scenario and waits for the child to report it is listening.
func startSignalChild(t *testing.T, scenario string) *signalChild {
	t.Helper()
	// #nosec G204 -- re-executes the test binary (os.Args[0]) for subprocess testing
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSignalChild$")
	cmd.Env = append(os.Environ(), signalScenarioEnv+"="+scenario)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	c := &signalChild{cmd: cmd, lines: make(chan string, 64)}
	go func() {
		defer close(c.lines)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if line, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				c.lines <- line
			}
		}
		_, _ = io.Copy(io.Discard, stdout)
	}()
	c.expect(t, "ready")
	return c
}

// expect requires the next events from the child to be want, in order.
func (c *signalChild) expect(t *testing.T, want ...string) {
	t.Helper()
	for _, w := range want {
		select {
		case got, ok := <-c.lines:
			require.True(t, ok, "child exited before reporting %q", w)
			require.Equal(t, w, got)
		case <-time.After(lineTimeout):
			require.FailNow(t, "timed out waiting for child event", "want %q", w)
		}
	}
}

// expectSet requires the next len(want) events to be want in any order.
func (c *signalChild) expectSet(t *testing.T, want ...string) {
	t.Helper()
	got := make([]string, 0, len(want))
	for range want {
		select {
		case line, ok := <-c.lines:
			require.True(t, ok, "child exited after %v, want %v", got, want)
			got = append(got, line)
		case <-time.After(lineTimeout):
			require.FailNow(t, "timed out waiting for child events", "got %v, want %v", got, want)
		}
	}
	require.ElementsMatch(t, want, got)
}

// kill delivers sig to the child with kill(2).
func (c *signalChild) kill(t *testing.T, sig syscall.Signal) {
	t.Helper()
	require.NoError(t, syscall.Kill(c.cmd.Process.Pid, sig))
}

// wait reaps the child and returns its wait status.
func (c *signalChild) wait(t *testing.T) syscall.WaitStatus {
	t.Helper()
	for range c.lines {
		// Drain so the reader goroutine finishes before Wait closes the pipe.
	}
	err := c.cmd.Wait()
	if err != nil {
		_, ok := errors.AsType[*exec.ExitError](err)
		require.True(t, ok, "wait: %v", err)
	}
	ws, ok := c.cmd.ProcessState.Sys().(syscall.WaitStatus)
	require.True(t, ok)
	return ws
}

// requireCleanExit requires the child to exit with status 0.
func (c *signalChild) requireCleanExit(t *testing.T) {
	t.Helper()
	ws := c.wait(t)
	require.True(t, ws.Exited(), "child did not exit normally: %v", ws)
	require.Zero(t, ws.ExitStatus())
}

func TestSignals_PriorityOrderAndRouting(t *testing.T) {
	t.Parallel()

	c := startSignalChild(t, "priority")

	c.kill(t, syscall.SIGUSR1)
	c.expect(t, "usr1-highest", "broadcast-high:user defined signal 1", "usr1-normal", "usr1-low")

	// SIGHUP reaches its own handler and the broadcast handler only.
	c.kill(t, syscall.SIGHUP)
	c.expect(t, "broadcast-high:hangup", "hup")

	c.kill(t, syscall.SIGTERM)
	c.expect(t, "term", "broadcast-high:terminated", "hook deadline=true", "shutdown err=<nil>")
	c.requireCleanExit(t)
}

func TestSignals_HandlerTimeoutAndShutdownTimeout(t *testing.T) {
	t.Parallel()

	c := startSignalChild(t, "timeouts")

	c.kill(t, syscall.SIGUSR1)
	c.expect(t,
		"stuck started",
		"error timeout=true",
		"hook ctxerr=<nil>",
		"shutdown deadline=true",
		"shutdown bounded=true",
	)
	c.requireCleanExit(t)
}

func TestSignals_ConcurrentDeliveriesOverlap(t *testing.T) {
	t.Parallel()

	c := startSignalChild(t, "concurrent")

	c.kill(t, syscall.SIGUSR1)
	c.expect(t, "start 1")
	// The first dispatch still blocks in its handler, so the second
	// delivery must run on another worker rather than queue behind it.
	c.kill(t, syscall.SIGUSR1)
	c.expect(t, "start 2")
	c.expectSet(t, "overlap", "overlap")
	c.expect(t, "shutdown err=<nil>")
	c.requireCleanExit(t)
}

func TestSignals_ParallelModeRunsHandlersConcurrently(t *testing.T) {
	t.Parallel()

	c := startSignalChild(t, "parallel")

	c.kill(t, syscall.SIGUSR2)
	c.expectSet(t, "a met b", "b met a")
	c.expect(t, "shutdown err=<nil>")
	c.requireCleanExit(t)
}

func TestSignals_SignalAfterShutdownUsesDefaultDisposition(t *testing.T) {
	t.Parallel()

	c := startSignalChild(t, "after-shutdown")

	c.kill(t, syscall.SIGTERM)
	c.expect(t, "term", "shutdown err=<nil>", "stopped")

	// Shutdown unsubscribed the channel, so the second SIGTERM gets the
	// default action and terminates the child instead of reaching the
	// handler.
	c.kill(t, syscall.SIGTERM)
	ws := c.wait(t)
	require.True(t, ws.Signaled(), "child was not terminated by the signal: %v", ws)
	require.Equal(t, syscall.SIGTERM, ws.Signal())
}

// TestSignalChild is the child half of the scenarios above. It does
// nothing unless the parent selected a scenario through the environment.
func TestSignalChild(t *testing.T) {
	scenario := os.Getenv(signalScenarioEnv)
	if scenario == "" {
		t.Skip("child entry point of the signal integration tests")
	}
	run, ok := childScenarios[scenario]
	require.True(t, ok, "unknown scenario %q", scenario)
	run(t)
}

// childScenarios maps a scenario name to the child body that runs it.
var childScenarios = map[string]func(t *testing.T){
	"priority":       childPriority,
	"timeouts":       childTimeouts,
	"concurrent":     childConcurrent,
	"parallel":       childParallel,
	"after-shutdown": childAfterShutdown,
}

// eventWriter serializes event lines so concurrent handlers never
// interleave partial lines on stdout.
var eventWriter sync.Mutex

func emit(format string, args ...any) {
	eventWriter.Lock()
	defer eventWriter.Unlock()
	fmt.Fprintf(os.Stdout, "event: "+format+"\n", args...)
}

// noHooks replaces the process-wide shutdown hooks so a scenario never
// runs hooks registered by other tests in the binary.
func noHooks(context.Context) error { return nil }

func childPriority(t *testing.T) {
	quit := make(chan struct{})
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1, syscall.SIGHUP, syscall.SIGTERM),
		signals.WithShutdownTimeout(lineTimeout),
		signals.WithShutdownHooks(func(ctx context.Context) error {
			_, ok := ctx.Deadline()
			emit("hook deadline=%t", ok)
			return nil
		}),
	)
	record := func(name string) signals.ContextHandler {
		return func(context.Context, os.Signal) error {
			emit("%s", name)
			return nil
		}
	}
	s.AddHandlerWithPriority(record("usr1-low"), signals.PriorityLow, syscall.SIGUSR1)
	s.AddHandler(record("usr1-normal"), syscall.SIGUSR1)
	s.AddHandlerWithPriority(record("usr1-highest"), signals.PriorityHighest, syscall.SIGUSR1)
	s.AddBroadcastHandlerWithPriority(func(_ context.Context, sig os.Signal) error {
		emit("broadcast-high:%v", sig)
		return nil
	}, signals.PriorityHigh)
	s.AddHandler(record("hup"), syscall.SIGHUP)
	s.AddHandlerWithPriority(func(context.Context, os.Signal) error {
		emit("term")
		close(quit)
		return nil
	}, signals.PriorityHighest, syscall.SIGTERM)

	s.Start()
	emit("ready")
	<-quit
	emit("shutdown err=%v", s.Shutdown(t.Context()))
}

func childTimeouts(*testing.T) {
	const (
		handlerTimeout  = 100 * time.Millisecond
		shutdownTimeout = 300 * time.Millisecond
	)
	started := make(chan struct{})
	reported := make(chan struct{})
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithHandlerTimeout(handlerTimeout),
		signals.WithShutdownTimeout(shutdownTimeout),
		signals.WithErrorHandler(func(_ os.Signal, err error) {
			emit("error timeout=%t", signals.IsTimeout(err))
			close(reported)
		}),
		signals.WithShutdownHooks(func(ctx context.Context) error {
			emit("hook ctxerr=%v", ctx.Err())
			return nil
		}),
	)
	// The handler ignores its context, so it outlives both its own
	// timeout and the shutdown deadline.
	s.AddHandler(func(context.Context, os.Signal) error {
		emit("stuck started")
		close(started)
		select {}
	}, syscall.SIGUSR1)

	s.Start()
	emit("ready")
	<-started
	<-reported

	begin := time.Now()
	err := s.Shutdown(context.Background())
	elapsed := time.Since(begin)
	emit("shutdown deadline=%t", errors.Is(err, context.DeadlineExceeded))
	emit("shutdown bounded=%t", elapsed >= shutdownTimeout/2 && elapsed < lineTimeout)
	// The stuck handler goroutine never returns; leave without waiting
	// for it so the child does not hang in the testing framework.
	os.Exit(0)
}

func childConcurrent(t *testing.T) {
	var running atomic.Int32
	both := make(chan struct{})
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR1),
		signals.WithShutdownHooks(noHooks),
	)
	s.AddHandler(func(context.Context, os.Signal) error {
		n := running.Add(1)
		emit("start %d", n)
		if n == 2 {
			close(both)
		}
		select {
		case <-both:
			emit("overlap")
		case <-time.After(lineTimeout):
			emit("alone")
		}
		return nil
	}, syscall.SIGUSR1)

	s.Start()
	emit("ready")
	<-both
	emit("shutdown err=%v", s.Shutdown(t.Context()))
}

func childParallel(t *testing.T) {
	aIn, bIn := make(chan struct{}), make(chan struct{})
	var done sync.WaitGroup
	done.Add(2)
	s := signals.New(
		signals.WithSignals(syscall.SIGUSR2),
		signals.WithExecutionMode(signals.ParallelMode),
		signals.WithShutdownHooks(noHooks),
	)
	// Each handler waits for the other to start: in sequential mode the
	// first would wait alone until the bound expires.
	meet := func(self string, mine, other chan struct{}, peer string) signals.ContextHandler {
		return func(context.Context, os.Signal) error {
			defer done.Done()
			close(mine)
			select {
			case <-other:
				emit("%s met %s", self, peer)
			case <-time.After(lineTimeout):
				emit("%s alone", self)
			}
			return nil
		}
	}
	s.AddHandlerWithPriority(meet("a", aIn, bIn, "b"), signals.PriorityHigh, syscall.SIGUSR2)
	s.AddHandlerWithPriority(meet("b", bIn, aIn, "a"), signals.PriorityLow, syscall.SIGUSR2)

	s.Start()
	emit("ready")
	done.Wait()
	emit("shutdown err=%v", s.Shutdown(t.Context()))
}

func childAfterShutdown(t *testing.T) {
	quit := make(chan struct{})
	s := signals.New(
		signals.WithSignals(syscall.SIGTERM),
		signals.WithShutdownHooks(noHooks),
	)
	s.AddHandler(func(context.Context, os.Signal) error {
		emit("term")
		close(quit)
		return nil
	}, syscall.SIGTERM)

	s.Start()
	emit("ready")
	<-quit
	emit("shutdown err=%v", s.Shutdown(t.Context()))
	emit("stopped")
	// Wait for the parent's second SIGTERM to terminate the process; the
	// bound turns a dispatcher that kept the subscription into a failure.
	time.Sleep(lineTimeout)
	emit("survived")
	os.Exit(1)
}

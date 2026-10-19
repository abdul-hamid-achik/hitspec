package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

// notifyInterrupts subscribes to SIGINT and SIGTERM and returns the channel and
// the function that unsubscribes. It is a variable so tests can inject a fake
// signal source.
var notifyInterrupts = func() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	return ch, func() { signal.Stop(ch) }
}

// exitNow terminates the process immediately; a variable for tests.
var exitNow = os.Exit

// interruptContext returns a context that is cancelled by the first signal
// received on sigs. The caller is expected to wind down gracefully (print the
// partial results, write the report) and exit ExitInterrupted. A second signal
// means the user does not want to wait: force is called at once.
//
// interrupted reports whether a signal was seen. stop releases the goroutine
// and must be called when the run ends.
func interruptContext(parent context.Context, sigs <-chan os.Signal, stderr io.Writer, force func(code int)) (ctx context.Context, interrupted func() bool, stop func()) {
	ctx, cancel := context.WithCancel(parent)
	var seen atomic.Bool
	done := make(chan struct{})
	go func() {
		select {
		case <-sigs:
		case <-done:
			return
		}
		seen.Store(true)
		fmt.Fprintln(stderr, "\nInterrupted: stopping and reporting what finished (press Ctrl+C again to exit now)")
		cancel()
		select {
		case <-sigs:
			force(ExitInterrupted)
		case <-done:
		}
	}()
	var stopped atomic.Bool
	return ctx, seen.Load, func() {
		if stopped.CompareAndSwap(false, true) {
			close(done)
		}
		cancel()
	}
}

package rill

import (
	"context"
	"sync"
	"sync/atomic"
)

type sinkOptions struct {
	settleFuncs []func()
}

func (o sinkOptions) settle() {
	for _, fn := range o.settleFuncs {
		fn()
	}
}

// A SinkOption is an optional argument accepted by every sink, such as a
// [Scope]. The interface cannot be implemented outside this package.
type SinkOption interface {
	apply(options *sinkOptions)
}

type sinkOptionFunc func(options *sinkOptions)

func (f sinkOptionFunc) apply(options *sinkOptions) {
	f(options)
}

func collectSinkOptions(options []SinkOption) sinkOptions {
	var result sinkOptions
	for _, option := range options {
		if option == nil {
			continue
		}
		option.apply(&result)
	}
	return result
}

// Settlement reports when the pipeline has no callbacks left to run. It
// returns the signal channel and the option that enables it. The option
// must not be reused across sinks.
//
// A pipeline settles when every callback started by the sink and its
// upstream stages has returned, including the ones that kept running
// after the result was known. No callback runs after that.
//
// Settlement does not stop the pipeline: unless the source is stopped
// first, the channel is closed only after the entire input is consumed.
// With an infinite source, that never happens.
func Settlement() (settled <-chan struct{}, opt SinkOption) {
	ch := make(chan struct{})
	settled = ch

	var cnt atomic.Int32

	opt = sinkOptionFunc(func(options *sinkOptions) {
		// This fires when the options are opened, i.e. at sink return rather
		// than at the call site. Detecting reuse at sink entry would require
		// re-plumbing every sink chain - too costly in the current architecture
		// for what it buys.
		if cnt.Add(1) > 1 {
			panic("rill: the same settlement option used more than once")
		}

		options.settleFuncs = append(options.settleFuncs, func() { close(ch) })
	})
	return
}

// A Scope tracks the lifecycle of a pipeline and lets the caller wait until
// all of its work is done - including any work that happens after a sink's
// early return.
//
// A scope passed to a sink as a [SinkOption] tracks not only that sink, but
// also the whole pipeline behind it.
//
// For branching pipelines (see [Tee]), multiple sinks can be attached to the
// same scope.
type Scope interface {
	SinkOption

	// Cancel cancels the scope's Context. It does not wait for the work
	// to finish.
	Cancel()

	// Wait is called after the sink has returned. It cancels the scope's
	// Context and blocks until the pipeline has settled: every user callback
	// has returned and any other remaining work is finished.
	//
	// Wait can be called any number of times and from multiple goroutines,
	// but only after every sink attached to the scope has returned.
	// Attaching a new sink to the scope after Wait has been called panics.
	Wait()
}

type scope struct {
	cancelFunc context.CancelFunc

	mu         sync.Mutex
	cond       sync.Cond
	cnt        int
	waitCalled bool
}

func (s *scope) apply(options *sinkOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Any join after Wait was called means Wait was called too early: the sink
	// had not returned yet, so Wait could not have covered it.
	if s.waitCalled {
		panic("rill: Wait must be called after all sinks attached to the scope have returned")
	}

	s.cnt++
	options.settleFuncs = append(options.settleFuncs, s.release)
}

func (s *scope) release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cnt--
	s.cond.Broadcast()
}

func (s *scope) Cancel() {
	s.cancelFunc()
}

func (s *scope) Wait() {
	s.cancelFunc()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.waitCalled = true
	for s.cnt > 0 {
		s.cond.Wait()
	}
}

// WithContext creates a new [Scope] and an associated Context derived from ctx.
//
// The Context is canceled by [Scope.Cancel] or [Scope.Wait]. At least one
// of them must be called, typically via defer, to prevent the Context from
// leaking.
func WithContext(ctx context.Context) (context.Context, Scope) {
	ctx, cancel := context.WithCancel(ctx)

	s := &scope{cancelFunc: cancel}
	s.cond.L = &s.mu

	return ctx, s
}

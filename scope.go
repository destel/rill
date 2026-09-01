package rill

import (
	"context"
	"sync"
)

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
	s.cnt--
	s.mu.Unlock()

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

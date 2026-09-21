package rill

import (
	"context"
	"sync/atomic"
)

type sinkOptions struct {
	onSettled      []func()
	onOutcomeKnown []func()
	waitForDrain   bool
}

func call(fns []func()) {
	for _, fn := range fns {
		fn()
	}
}

// A SinkOption is an optional argument accepted by every sink.
type SinkOption interface {
	apply(options *sinkOptions)
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

type sinkOptionFunc func(options *sinkOptions)

func (f sinkOptionFunc) apply(options *sinkOptions) {
	f(options)
}

// WithContext returns a context derived from ctx and a SinkOption.
// A sink given this option cancels the context as soon as its outcome
// is known, then waits for the pipeline to finish before returning.
//
// The returned option must be used exactly once: using it more than once
// panics, and never using it can leak the context. In particular, it can't
// cover branching pipelines where every branch ends with its own sink.
func WithContext(ctx context.Context) (context.Context, SinkOption) {
	var cnt atomic.Int32

	ctx, cancel := context.WithCancel(ctx)

	ops := sinkOptionFunc(func(options *sinkOptions) {
		if cnt.Add(1) > 1 {
			panic("rill: WithContext option must be passed to exactly one sink")
		}

		options.onOutcomeKnown = append(options.onOutcomeKnown, cancel)
		options.waitForDrain = true
	})

	return ctx, ops
}

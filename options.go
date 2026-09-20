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
// [WithContext] returns one.
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

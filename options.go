package rill

import "sync/atomic"

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

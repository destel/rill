package rill

import "sync/atomic"

type sinkOptions struct {
	settleChans []chan struct{}
}

func (o sinkOptions) settle() {
	for _, ch := range o.settleChans {
		close(ch)
	}
}

// A SinkOption is an optional argument accepted by every sink, such as the
// one returned by [Settlement]. The interface cannot be implemented outside
// this package.
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
		if cnt.Add(1) > 1 {
			panic("rill: settlement option must not be reused across sinks")
		}

		options.settleChans = append(options.settleChans, ch)
	})
	return
}

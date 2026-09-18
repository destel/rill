package rill

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
// [Scope] implements this interface.
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

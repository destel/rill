package rill

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

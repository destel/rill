package rill

import (
	"github.com/destel/rill/internal/core"
)

// Map takes a stream of values of type A and returns a stream of values of
// type B, using f to transform each. When f returns an error, it's written
// to the output instead of a value.
//
// The argument n bounds the number of concurrent calls to f. Results are
// written to the output as they become ready, so their order can differ
// from the input order when n > 1. Use [OrderedMap] to preserve the order.
//
// See the package documentation for the behaviors that all stages share.
func Map[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	validateN(n)
	validateNilFunc(f == nil)

	return core.FilterMap(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, err := f(a.Value)
		if err != nil {
			return Try[B]{Error: err}, true
		}

		return Try[B]{Value: b}, true
	})
}

// OrderedMap is the ordered version of [Map]: the output preserves the
// input order, for values and errors alike.
func OrderedMap[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	validateN(n)
	validateNilFunc(f == nil)

	return core.OrderedFilterMap(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, err := f(a.Value)
		if err != nil {
			return Try[B]{Error: err}, true
		}

		return Try[B]{Value: b}, true
	})
}

// Filter takes a stream of values and returns a new stream, keeping
// only the values that match the condition f. When f returns an error,
// it's written to the output instead of the value.
//
// The argument n bounds the number of concurrent calls to f. Results are
// written to the output as they become ready, so their order can differ
// from the input order when n > 1. Use [OrderedFilter] to preserve the
// order.
//
// See the package documentation for the behaviors that all stages share.
func Filter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	validateN(n)
	validateNilFunc(f == nil)

	return core.FilterMap(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error != nil {
			return a, true // never filter out errors
		}

		keep, err := f(a.Value)
		if err != nil {
			return Try[A]{Error: err}, true // never filter out errors
		}

		return a, keep
	})
}

// OrderedFilter is the ordered version of [Filter]: the output preserves
// the input order, for values and errors alike.
func OrderedFilter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	validateN(n)
	validateNilFunc(f == nil)

	return core.OrderedFilterMap(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error != nil {
			return a, true // never filter out errors
		}

		keep, err := f(a.Value)
		if err != nil {
			return Try[A]{Error: err}, true // never filter out errors
		}

		return a, keep
	})
}

// FilterMap takes a stream of values of type A and returns a stream of
// values of type B, using f to transform each value and decide whether
// to keep the result. When f returns an error, it's written to the
// output instead of a value.
//
// The argument n bounds the number of concurrent calls to f. Results are
// written to the output as they become ready, so their order can differ
// from the input order when n > 1. Use [OrderedFilterMap] to preserve
// the order.
//
// See the package documentation for the behaviors that all stages share.
func FilterMap[A, B any](in <-chan Try[A], n int, f func(A) (B, bool, error)) <-chan Try[B] {
	validateN(n)
	validateNilFunc(f == nil)

	return core.FilterMap(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, keep, err := f(a.Value)
		if err != nil {
			return Try[B]{Error: err}, true
		}

		return Try[B]{Value: b}, keep
	})
}

// OrderedFilterMap is the ordered version of [FilterMap]: the output
// preserves the input order, for values and errors alike.
func OrderedFilterMap[A, B any](in <-chan Try[A], n int, f func(A) (B, bool, error)) <-chan Try[B] {
	validateN(n)
	validateNilFunc(f == nil)

	return core.OrderedFilterMap(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, keep, err := f(a.Value)
		if err != nil {
			return Try[B]{Error: err}, true
		}

		return Try[B]{Value: b}, keep
	})
}

// FlatMap takes a stream of values of type A and returns a stream of
// values of type B, using f to expand each value into its own sub-stream.
// The sub-streams are flattened into the output: every item is forwarded,
// values and errors alike.
//
// The argument n bounds the number of sub-streams consumed concurrently:
// each worker consumes one sub-stream to the end before starting the next.
// When n > 1, items from different sub-streams can interleave in the
// output. Use [OrderedFlatMap] to concatenate the sub-streams in the
// input order.
//
// See the package documentation for the behaviors that all stages share.
func FlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	validateN(n)
	validateNilFunc(f == nil)

	if in == nil {
		return nil
	}

	out := make(chan Try[B])

	core.Loop(in, out, n, func(a Try[A]) {
		if a.Error != nil {
			out <- Try[B]{Error: a.Error}
			return
		}

		bb := f(a.Value)
		for b := range bb {
			out <- b
		}
	})

	return out
}

// OrderedFlatMap is the ordered version of [FlatMap]: the output is the
// sub-streams concatenated in the input order.
//
// The argument n bounds the number of concurrent calls to f. The
// sub-streams are prepared concurrently, but - unlike in [FlatMap] -
// consumed one at a time and in order: nothing reads from a sub-stream
// before its turn. In practice, to keep the stage concurrent, a
// sub-stream must do all or part of its expensive work ahead of its
// turn.
//
// Consider a stream of URLs: each file should be downloaded, and its
// lines streamed to the output, all in order. Downloading is the
// expensive work here.
//
// Example 1: f downloads the whole file into memory, then streams the
// lines from there. Up to 5 downloads run concurrently.
//
//	rill.OrderedFlatMap(urls, 5, func(u string) <-chan rill.Try[string] {
//		lines, err := getFileLines(u)
//		return rill.FromSlice(lines, err)
//	})
//
// Example 2: f streams the lines as the file is being downloaded,
// through a [Buffer] that lets the sub-stream run ahead of its turn.
// Again up to 5 concurrent downloads, but each pauses after the first
// 100 lines, until its turn comes.
//
//	rill.OrderedFlatMap(urls, 5, func(u string) <-chan rill.Try[string] {
//		lines := streamFileLines(u)
//		return rill.Buffer(lines, 100)
//	})
//
// The two examples do the same thing: they buffer the lines, with or
// without a bound. Without any buffering, the downloads would run one
// at a time, and the stage would turn sequential.
//
// See the package documentation for the behaviors that all stages share.
func OrderedFlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	validateN(n)
	validateNilFunc(f == nil)

	if in == nil {
		return nil
	}

	out := make(chan Try[B])

	core.OrderedLoop(in, out, n, func(a Try[A], canWrite <-chan struct{}) {
		if a.Error != nil {
			<-canWrite
			out <- Try[B]{Error: a.Error}
			return
		}

		bb := f(a.Value)
		<-canWrite
		for b := range bb {
			out <- b
		}
	})

	return out
}

// Catch takes a stream and returns a new stream with the errors
// optionally handled by f. Each error is passed to f, which returns nil
// to drop it from the stream, the same error to keep it, or a different
// one to replace it. Values never reach f and are passed through as-is.
//
// The argument n bounds the number of concurrent calls to f. Items are
// written to the output as they become ready, so their order can differ
// from the input order when n > 1. Use [OrderedCatch] to preserve the
// order.
//
// See the package documentation for the behaviors that all stages share.
func Catch[A any](in <-chan Try[A], n int, f func(error) error) <-chan Try[A] {
	validateN(n)
	validateNilFunc(f == nil)

	return core.FilterMap(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error == nil {
			return a, true
		}

		err := f(a.Error)
		if err == nil {
			return a, false // error handled, filter out
		}

		return Try[A]{Error: err}, true // error replaced by f(a.Error)
	})
}

// OrderedCatch is the ordered version of [Catch]: the output preserves
// the input order, for values and errors alike.
func OrderedCatch[A any](in <-chan Try[A], n int, f func(error) error) <-chan Try[A] {
	validateN(n)
	validateNilFunc(f == nil)

	return core.OrderedFilterMap(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error == nil {
			return a, true
		}

		err := f(a.Error)
		if err == nil {
			return a, false // error handled, filter out
		}

		return Try[A]{Error: err}, true // error replaced by f(a.Error)
	})
}

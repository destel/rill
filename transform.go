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

// FlatMap takes a stream of items of type A and transforms each item into a new sub-stream of items of type B using a function f.
// Those sub-streams are then flattened into a single output stream, which is returned.
//
// This is a non-blocking unordered function that processes items concurrently using n goroutines.
// An ordered version of this function, [OrderedFlatMap], is also available.
//
// See the package documentation for more information on non-blocking unordered functions and error handling.
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

// OrderedFlatMap is the ordered version of [FlatMap].
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

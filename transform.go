package rill

import (
	"github.com/destel/rill/internal/core"
)

// Map applies a transformation function to each item in an input channel, using n goroutines for concurrency.
// If an error is encountered, either from the function f itself or from upstream it is forwarded to the output for further handling.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use [OrderedMap] to preserve the input order.
func Map[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	return core.MapOrFilter(in, n, func(a Try[A]) (Try[B], bool) {
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

// OrderedMap is similar to [Map], but it guarantees that the output order is the same as the input order.
func OrderedMap[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	return core.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[B], bool) {
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

// Filter removes items that do not meet a specified condition, using n goroutines for concurrency.
// If an error is encountered, either from the function f itself or from upstream it is forwarded to the output for further handling.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use [OrderedFilter] to preserve the input order.
func Filter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	return core.MapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
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

// OrderedFilter is similar to [Filter], but it guarantees that the output order is the same as the input order.
func OrderedFilter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	return core.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
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

// FlatMap applies a function to each item in an input channel, where the function returns a channel of items.
// These items are then flattened into a single output channel using n goroutines for concurrency.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use [OrderedFlatMap] to preserve the input order.
func FlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
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

// OrderedFlatMap is similar to [FlatMap], but it guarantees that the output order is the same as the input order.
func OrderedFlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
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

// Catch allows handling errors from the input channel using n goroutines for concurrency.
// When f returns nil, error is considered handled and filtered out; otherwise it is replaced by the result of f.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use [OrderedCatch] to preserve the input order.
func Catch[A any](in <-chan Try[A], n int, f func(error) error) <-chan Try[A] {
	return core.MapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
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

// OrderedCatch is similar to [Catch], but it guarantees that the output order is the same as the input order.
func OrderedCatch[A any](in <-chan Try[A], n int, f func(error) error) <-chan Try[A] {
	return core.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
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

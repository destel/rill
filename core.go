package rill

import (
	"sync"

	"github.com/destel/rill/internal/common"
)

// Map applies a transformation function to each item in an input channel, using n goroutines for concurrency.
// If an error is encountered, either from the function f itself or from upstream it is forwarded to the output for further handling.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use [OrderedMap] to preserve the input order.
func Map[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	return common.MapOrFilter(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, err := f(a.V)
		if err != nil {
			return Try[B]{Error: err}, true
		}

		return Try[B]{V: b}, true
	})
}

// OrderedMap is similar to [Map], but it guarantees that the output order is the same as the input order.
func OrderedMap[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	return common.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, err := f(a.V)
		if err != nil {
			return Try[B]{Error: err}, true
		}

		return Try[B]{V: b}, true
	})
}

// Filter removes items that do not meet a specified condition, using n goroutines for concurrency.
// If an error is encountered, either from the function f itself or from upstream it is forwarded to the output for further handling.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use [OrderedFilter] to preserve the input order.
func Filter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	return common.MapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error != nil {
			return a, true // never filter out errors
		}

		keep, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, true // never filter out errors
		}

		return a, keep
	})
}

// OrderedFilter is similar to [Filter], but it guarantees that the output order is the same as the input order.
func OrderedFilter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	return common.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error != nil {
			return a, true // never filter out errors
		}

		keep, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, true // never filter out errors
		}

		return a, keep
	})
}

// FlatMap applies a function to each item in an input channel, where the function returns a channel of items.
// These items are then flattened into a single output channel. Uses n goroutines for concurrency.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use [OrderedFlatMap] to preserve the input order.
func FlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	if in == nil {
		return nil
	}

	out := make(chan Try[B])

	common.Loop(in, out, n, func(a Try[A]) {
		if a.Error != nil {
			out <- Try[B]{Error: a.Error}
			return
		}

		bb := f(a.V)
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

	common.OrderedLoop(in, out, n, func(a Try[A], canWrite <-chan struct{}) {
		if a.Error != nil {
			<-canWrite
			out <- Try[B]{Error: a.Error}
			return
		}

		bb := f(a.V)
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
	return common.MapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
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
	return common.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
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

// ForEach applies a function f to each item in an input channel using n goroutines for parallel processing. The function
// blocks until all items are processed or an error is encountered, either from the function f itself or from upstream.
// In case of an error leading to early termination, ForEach ensures the input channel is drained to avoid goroutine leaks,
// making it safe for use in environments where cleanup is crucial. The function returns the first encountered error, or nil
// if all items were processed successfully.
// While this function does not guarantee the order of item processing due to its concurrent nature,
// using n = 1 results in sequential processing, as in a simple for-range loop.
func ForEach[A any](in <-chan Try[A], n int, f func(A) error) error {
	if n == 1 {
		for a := range in {
			err := a.Error
			if err == nil {
				err = f(a.V)
			}

			if err != nil {
				DrainNB(in)
				return err
			}
		}

		return nil
	}

	var retErr error
	var once sync.Once

	in, earlyExit := common.Breakable(in)
	done := make(chan struct{})

	common.Loop(in, done, n, func(a Try[A]) {
		err := a.Error
		if err == nil {
			err = f(a.V)
		}

		if err != nil {
			earlyExit()
			once.Do(func() {
				retErr = err
			})
		}
	})

	<-done
	return retErr
}

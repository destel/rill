package echans

import (
	"sync"

	"github.com/destel/rill/chans"
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
	return common.MapOrFlatMap(in, n, func(a Try[A]) (b Try[B], bb <-chan Try[B], flat bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, nil, false
		}
		return Try[B]{}, f(a.V), true
	})
}

// OrderedFlatMap is similar to [FlatMap], but it guarantees that the output order is the same as the input order.
func OrderedFlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	return common.OrderedMapOrFlatMap(in, n, func(a Try[A]) (b Try[B], bb <-chan Try[B], flat bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, nil, false
		}
		return Try[B]{}, f(a.V), true
	})
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
	var retErr error
	var once sync.Once

	chans.ForEach(in, n, func(a Try[A]) bool {
		err := a.Error
		if err == nil {
			err = f(a.V)
		}

		if err != nil {
			once.Do(func() {
				retErr = err
			})
			return false // early exit
		}

		return true
	})

	return retErr
}

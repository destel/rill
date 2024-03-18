package chans

import "github.com/destel/rill/internal/common"

// Map applies a transformation function to each item in an input channel, using n goroutines for concurrency.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use OrderedMap to preserve the input order.
func Map[A, B any](in <-chan A, n int, f func(A) B) <-chan B {
	return common.MapOrFilter(in, n, func(a A) (B, bool) {
		return f(a), true
	})
}

// OrderedMap is similar to Map, but it guarantees that the output order is the same as the input order.
func OrderedMap[A, B any](in <-chan A, n int, f func(A) B) <-chan B {
	return common.OrderedMapOrFilter(in, n, func(a A) (B, bool) {
		return f(a), true
	})
}

// Filter removes items that do not meet a specified condition, using n goroutines for concurrent processing.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use OrderedFilter to preserve the input order.
func Filter[A any](in <-chan A, n int, f func(A) bool) <-chan A {
	return common.MapOrFilter(in, n, func(a A) (A, bool) {
		return a, f(a)
	})
}

// OrderedFilter is similar to Filter, but it guarantees that the output order is the same as the input order.
func OrderedFilter[A any](in <-chan A, n int, f func(A) bool) <-chan A {
	return common.OrderedMapOrFilter(in, n, func(a A) (A, bool) {
		return a, f(a)
	})
}

// FlatMap applies a function to each item in an input channel, where the function returns a channel of items.
// These items are then flattened into a single output channel. Uses n goroutines for concurrency.
// The output order is not guaranteed: results are written to the output as soon as they're ready.
// Use OrderedFlatMap to preserve the input order.
func FlatMap[A, B any](in <-chan A, n int, f func(A) <-chan B) <-chan B {
	var zero B
	return common.MapOrFlatMap(in, n, func(a A) (b B, bb <-chan B, flat bool) {
		return zero, f(a), true
	})
}

// OrderedFlatMap is similar to FlatMap, but it guarantees that the output order is the same as the input order.
func OrderedFlatMap[A, B any](in <-chan A, n int, f func(A) <-chan B) <-chan B {
	var zero B
	return common.OrderedMapOrFlatMap(in, n, func(a A) (b B, bb <-chan B, flat bool) {
		return zero, f(a), true
	})
}

// ForEach applies a function to each item in an input channel using n goroutines. The function blocks until
// all items are processed or the function returns false. In case of early termination, ForEach ensures
// the input channel is drained to avoid goroutine leaks, making it safe for use in environments where cleanup is crucial.
// While this function does not guarantee the order of item processing due to its concurrent nature,
// using n = 1 results in sequential processing, as in a simple for-range loop.
func ForEach[A any](in <-chan A, n int, f func(A) bool) {
	if n == 1 {
		for a := range in {
			if !f(a) {
				DrainNB(in)
				break
			}
		}

		return
	}

	in, earlyExit := common.Breakable(in)
	done := make(chan struct{})

	common.Loop(in, done, n, func(a A) {
		if !f(a) {
			earlyExit()
		}
	})

	<-done
}

package rill

import (
	"sync"

	"github.com/destel/rill/internal/core"
)

// Reduce combines all items from the input stream into a single value
// using a binary function f. The function f must be commutative, meaning
// f(x,y) == f(y,x). It is applied to pairs of items, progressively reducing the stream's
// contents until only one value remains.
//
// Reduce blocks until one of the following conditions is met:
//   - An error is encountered in the stream - Reduce returns the error.
//   - Function f returns an error - Reduce returns the error.
//   - The end of the stream is reached, and it was empty - Reduce sets the hasResult flag to false.
//   - The end of the stream is reached, and it was not empty - Reduce returns the result and sets the hasResult flag to true.
//
// Reduce uses n goroutines to call the user-provided function f concurrently, which means that the
// order in which the function f is applied is undefined. That's the reason why function f must be commutative.
//
// If Reduce terminates early (before reaching the end of the input stream), it initiates
// background draining of the remaining items. This is done to prevent goroutine
// leaks by ensuring that all goroutines feeding the stream are allowed to complete.
// The input stream should not be used anymore after calling this function.
func Reduce[A any](in <-chan Try[A], n int, f func(A, A) (A, error)) (result A, hasResult bool, err error) {
	in, earlyExit := core.Breakable(in)

	res, ok := core.Reduce(in, n, func(a1, a2 Try[A]) Try[A] {
		if err := a1.Error; err != nil {
			earlyExit()
			return a1
		}

		if err := a2.Error; err != nil {
			earlyExit()
			return a2
		}

		res, err := f(a1.Value, a2.Value)
		if err != nil {
			earlyExit()
			return Try[A]{Error: err}
		}

		return Try[A]{Value: res}
	})

	return res.Value, ok, res.Error
}

// MapReduce transforms the input stream into a map using a mapper and a reducer functions.
// The transformation is performed in two concurrent phases.
//
//   - The mapper function transforms each input item into a key-value pair.
//   - The reducer function reduces values for the same key into a single value.
//     This phase hase the same semantics as the [Reduce] function, in particular
//     the reducer function must be commutative.
//
// MapReduce blocks until one of the following conditions is met:
//   - An error is encountered in the stream - MapReduce returns the error.
//   - mapper function returns an error - MapReduce returns the error.
//   - reducer function returns an error - MapReduce returns the error.
//   - The end of the stream is reached - MapReduce returns the result.
//
// MapReduce calls mapper and reducer functions concurrently using nm and nr goroutines respectively.
//
// If MapReduce terminates early (before reaching the end of the input stream), it initiates
// background draining of the remaining items. This is done to prevent goroutine
// leaks by ensuring that all goroutines feeding the stream are allowed to complete.
// The input stream should not be used anymore after calling this function.
func MapReduce[A any, K comparable, V any](in <-chan Try[A], nm int, mapper func(A) (K, V, error), nr int, reducer func(V, V) (V, error)) (map[K]V, error) {
	var zeroKey K
	var zeroVal V

	in, earlyExit := core.Breakable(in)

	var retErr error
	var once sync.Once

	reportError := func(err error) {
		earlyExit()
		once.Do(func() {
			retErr = err
		})
	}

	res := core.MapReduce(in,
		nm, func(a Try[A]) (K, V) {
			if a.Error != nil {
				reportError(a.Error)
				return zeroKey, zeroVal
			}

			k, v, err := mapper(a.Value)
			if err != nil {
				reportError(err)
				return zeroKey, zeroVal
			}

			return k, v
		},
		nr, func(v1, v2 V) V {
			res, err := reducer(v1, v2)
			if err != nil {
				reportError(err)
				return zeroVal
			}

			return res
		},
	)

	return res, retErr
}

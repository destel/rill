package rill

import (
	"sync"

	"github.com/destel/rill/internal/core"
)

// Reduce combines all elements from the input channel into a single value
// using a binary function f. The function f must be commutative, meaning
// f(x,y) == f(y,x). It is applied to pairs of elements, using n
// goroutines, progressively reducing the channel's contents until only one value remains.
// The order in which the function f is applied is not guaranteed due to concurrent processing.
//
// Reduce blocks until all items are processed or an error is encountered,
// either from the function f itself or from the upstream. In case of an error
// leading to early termination, Reduce ensures the input channel is drained to
// avoid goroutine leaks, making it safe for use in environments where cleanup
// is crucial.
//
// The function returns the first encountered error, if any, or the reduction result.
// The second return value is false if the input channel is empty, and true otherwise.
func Reduce[A any](in <-chan Try[A], n int, f func(A, A) (A, error)) (A, bool, error) {
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

// MapReduce reduces the input channel to a map using a mapper and a reducer functions.
// Reduction is done in two phases, both occurring concurrently. In the first phase,
// the mapper function transforms each input item into a key-value pair.
// As a result of this phase, we can get multiple values for the same key, so
// in the second phase, the reducer function reduces values for the same key into a single value.
// The order in which the reducer is applied is not guaranteed due to concurrent processing.
// See [Reduce] documentation for more details on reduction phase semantics.
//
// The number of concurrent mappers and reducers can be controlled using nm and nr parameters respectively.
//
// MapReduce blocks until all items are processed or an error is encountered,
// either from the mapper, reducer, or the upstream. In case of an error
// leading to early termination, MapReduce ensures the input channel is drained
// to avoid goroutine leaks, making it safe for use in environments where
// cleanup is crucial.
//
// The function returns the first encountered error, if any, or a map where
// each key is associated with a single reduced value
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

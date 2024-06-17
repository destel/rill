package rill

import (
	"sync"

	"github.com/destel/rill/internal/core"
)

// Reduce combines all items from the input stream into a single value using a binary function f.
// The function f is called for pairs of items, progressively reducing the stream contents until only one value remains.
//
// As an unordered function, Reduce can apply f to any pair of items in any order, which requires f to be:
//   - Associative: f(a, f(b, c)) == f(f(a, b), c)
//   - Commutative: f(a, b) == f(b, a)
//
// The hasResult return flag is set to false if the stream was empty, otherwise it is set to true.
//
// Reduce is a blocking unordered function that processes items concurrently using n goroutines.
// The case when n = 1 is optimized: it does not spawn additional goroutines and processes items sequentially,
// making the function ordered. This also removes the need for the function f to be commutative.
//
// See the package documentation for more information on blocking unordered functions and error handling.
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

// MapReduce transforms the input stream into a Go map using a mapper and a reducer functions.
// The transformation is performed in two concurrent phases.
//
//   - The mapper function transforms each input item into a key-value pair.
//   - The reducer function reduces values for the same key into a single value.
//     This phase has the same semantics as the [Reduce] function, in particular
//     the reducer function must be commutative and associative.
//
// MapReduce is a blocking unordered function that processes items concurrently using nm and nr goroutines
// for the mapper and reducer functions respectively. Setting nr = 1 will make the reduce phase sequential and ordered,
// see [Reduce] for more information.
//
// See the package documentation for more information on blocking unordered functions and error handling.
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

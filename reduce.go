package rill

import (
	"errors"
	"sync/atomic"

	"github.com/destel/rill/internal/core"
)

// lowLevelReduceStage transforms a stream of values into a stream with a
// single value - the fold of all input values using f.
//
// At n = 1 the final value is a single chain over the input in order: f(f(f(f(a1, a2), a3), a4), ...).
// For n > 1 there are multiple such chains working concurrently so reduction order is undefined, and
// the supplied callback must be commutative and associative.
//
//   - at most n calls to f run concurrently
//   - all input errors are forwarded as they occur
//   - the callback can emit any number of additional errors per call
//   - the result is emitted at the end, after all errors, and only if the input had at least one value
//   - what emitted errors imply about the result is the caller's contract
func lowLevelReduceStage[A any](in Stream[A], n int, f func(A, A, func(error)) A) Stream[A] {
	if n <= 1 {
		return Generate(func(send func(A), sendErr func(error)) {
			var acc A
			first := true

			for a := range in {
				if a.Error != nil {
					sendErr(a.Error)
					continue
				}

				if first {
					acc = a.Value
					first = false
					continue
				}

				res := f(acc, a.Value, sendErr)
				acc = res
			}

			if !first {
				send(acc)
			}
		})
	}

	chains := make([]Stream[A], n)
	for i := range chains {
		chains[i] = lowLevelReduceStage(in, 1, f)
	}

	partials := core.Merge(chains...)

	// partials contains at most n values - one per chain. Reducing them in
	// parallel takes at most n/2 concurrent f calls, so n/2 is all the
	// concurrency the next level can use. The total concurrency across all
	// levels still cannot exceed n: each concurrent f call at the next level
	// requires two values, and hence two finished chains, at this level.
	return lowLevelReduceStage(partials, n/2, f)
}

// lowLevelFoldStage is the seeded, sequential counterpart of
// lowLevelReduceStage. The final value is a single chain over the input
// in order: f(f(f(f(seed, a1), a2), a3), ...).
// Same error model: the callback emits errors instead of returning them,
// and always returns the value to carry forward.
//
// Unlike a reduce, for an empty input the final value is the seed.
func lowLevelFoldStage[S any, A any](in Stream[A], seed S, f func(S, A, func(error)) S) Stream[S] {
	return Generate(func(send func(S), sendErr func(error)) {
		acc := seed

		for v := range in {
			if v.Error != nil {
				sendErr(v.Error)
				continue
			}
			acc = f(acc, v.Value, sendErr)
		}

		send(acc)
	})
}

// reduceStage transforms a stream of values into a stream with a single
// value - the fold of all input values using the associative and commutative
// function f.
//
// It's lowLevelReduceStage with the conventional error handling; on top of
// its contract:
//   - f errors are emitted as they occur, exactly one per failed call
//   - f errors make the result partial: a fold of only some subset of the
//     values; the exact subset is only specified at n = 1 - the right
//     operands of all failed f calls are excluded from the fold
func reduceStage[A any](in Stream[A], n int, f func(A, A) (A, error)) Stream[A] {
	return lowLevelReduceStage(in, n, func(a1, a2 A, sendErr func(error)) A {
		res, err := f(a1, a2)
		if err != nil {
			sendErr(err)
			return a1 // the left operand is the carried value; dropping a2 is what the partial-result bullet promises
		}
		return res
	})
}

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
// When n = 1, items are processed sequentially in stream order, making the function ordered.
// This also removes the need for the function f to be associative and commutative.
//
// See the package documentation for more information on blocking unordered functions and error handling.
func Reduce[A any](in <-chan Try[A], n int, f func(A, A) (A, error)) (result A, hasResult bool, err error) {
	var zero A

	if n == 1 {
		var acc A
		seeded := false
		err := ForEach(in, 1, func(v A) error {
			if !seeded {
				acc = v
				seeded = true
				return nil
			}

			res, err := f(acc, v)
			if err != nil {
				return err
			}
			acc = res
			return nil
		})
		if err != nil {
			return zero, false, err
		}
		return acc, seeded, nil
	}

	var stopped atomic.Bool
	defer stopped.Store(true)

	out := reduceStage(in, n, func(a1, a2 A) (A, error) {
		if stopped.Load() {
			return a1, nil
		}
		return f(a1, a2)
	})

	return First(out)
}

// mergeIntoMap adds the (k, v) pair to the map m.
// If the key already exists, it is merged with the new value using the merge function.
// On merge failure the map is left unchanged and the error is returned.
func mergeIntoMap[K comparable, V any](m map[K]V, k K, v V, mergeFunc func(V, V) (V, error)) error {
	val, ok := m[k]
	if !ok {
		m[k] = v
		return nil
	}

	newVal, err := mergeFunc(val, v)
	if err != nil {
		return err
	}

	m[k] = newVal
	return nil
}

// errSkip is a control-flow sentinel, compared by identity - the fs.SkipDir pattern.
var errSkip = errors.New("skip")

// mapReduceStage transforms a stream of values into a stream with a single map.
//
// Semantically it's equivalent to transforming each value into a single-key map using mapper
// and then reducing those maps per key using reducer. The only difference is that empty input produces an empty map,
//
// nm and nr control the maximum number of concurrent mapper and reducer calls respectively.
func mapReduceStage[A any, K comparable, V any](in Stream[A], nm int, mapper func(A) (K, V, error), nr int, reducer func(V, V) (V, error)) Stream[map[K]V] {
	type keyValue struct {
		Key   K
		Value V
	}

	pairs := FilterMap(in, nm, func(a A) (keyValue, bool, error) {
		k, v, err := mapper(a)
		if err == errSkip { //nolint:errorlint
			return keyValue{}, false, nil
		}
		if err != nil {
			return keyValue{}, false, err
		}
		return keyValue{k, v}, true, nil
	})

	chains := make([]Stream[map[K]V], nr)
	for i := range chains {
		chains[i] = lowLevelFoldStage(pairs, make(map[K]V), func(m map[K]V, p keyValue, sendErr func(error)) map[K]V {
			if err := mergeIntoMap(m, p.Key, p.Value, reducer); err != nil {
				sendErr(err)
			}
			return m
		})
	}

	if len(chains) == 1 {
		return chains[0]
	}

	// The same argument as in lowLevelReduceStage bounds concurrent reducer
	// calls by nr: each concurrent merge at the next level requires two
	// finished fold chains.
	return lowLevelReduceStage(core.Merge(chains...), nr/2, func(m1, m2 map[K]V, sendErr func(error)) map[K]V {
		for k, v := range m2 {
			if err := mergeIntoMap(m1, k, v, reducer); err != nil {
				sendErr(err)
			}
		}
		return m1
	})
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
	var stopped atomic.Bool
	defer stopped.Store(true)

	var zeroK K
	var zeroV V

	out := mapReduceStage(in,
		nm, func(a A) (K, V, error) {
			if stopped.Load() {
				return zeroK, zeroV, errSkip
			}
			return mapper(a)
		},
		nr, func(v1, v2 V) (V, error) {
			if stopped.Load() {
				return v1, nil
			}
			return reducer(v1, v2)
		},
	)

	m, _, err := First(out)
	return m, err
}

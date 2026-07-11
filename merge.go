package rill

import (
	"github.com/destel/rill/internal/core"
)

// Merge performs a fan-in operation on the list of input channels, returning a single output channel.
// The resulting channel will contain all items from all inputs,
// and will be closed when all inputs are fully consumed.
//
// This is a non-blocking function that processes items from each input sequentially.
//
// See the package documentation for more information on non-blocking functions and error handling.
func Merge[A any](ins ...<-chan A) <-chan A {
	return core.Merge(ins...)
}

// Split2 divides the input stream into two output streams based on the predicate function f:
// The splitting behavior is determined by the boolean return value of f. When f returns true, the item is sent to the outTrue stream,
// otherwise it is sent to the outFalse stream. In case of any error, the item is sent to both output streams.
// Both output streams must be consumed independently to avoid deadlocks.
//
// This is a non-blocking unordered function that processes items concurrently using n goroutines.
// An ordered version of this function, [OrderedSplit2], is also available.
//
// See the package documentation for more information on non-blocking unordered functions and error handling.
//
// Deprecated: Split2 will be removed in v1.0. Since the introduction of [Tee]
// in v0.8, splitting no longer needs a dedicated operation — it can be composed
// from existing ones. Unlike Split2, the composition is also not limited to two
// branches.
//
// Quite often the predicate is a simple pure check (field comparison, type
// switch, etc). In such cases splitting is just [Tee] plus a [Filter] on each
// branch:
//
//	adults, minors := rill.Tee(users)
//	adults = rill.Filter(adults, 1, func(u User) (bool, error) { return u.Age >= 18, nil })
//	minors = rill.Filter(minors, 1, func(u User) (bool, error) { return u.Age < 18, nil })
//
// If the predicate is expensive, stateful, or can fail, it must be evaluated
// once per item, before [Tee]: inline this function's implementation, which
// tags each item with the decision and routes on the tag. The same pattern
// extends to n-way splitting by tagging with an index or key instead of a bool.
func Split2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	type Decision[A any] struct {
		Value    A
		Decision bool
	}

	tagged := Map(in, n, func(a A) (Decision[A], error) {
		d, err := f(a)
		return Decision[A]{Value: a, Decision: d}, err
	})

	tagged1, tagged2 := Tee(tagged)
	outTrue = FilterMap(tagged1, 1, func(d Decision[A]) (A, bool, error) { return d.Value, d.Decision, nil })
	outFalse = FilterMap(tagged2, 1, func(d Decision[A]) (A, bool, error) { return d.Value, !d.Decision, nil })

	return
}

// OrderedSplit2 is the ordered version of [Split2].
//
// Deprecated: OrderedSplit2 will be removed in v1.0. Since the introduction of
// [Tee] in v0.8, splitting no longer needs a dedicated operation — it can be
// composed from existing ones. Unlike OrderedSplit2, the composition is also
// not limited to two branches.
//
// Quite often the predicate is a simple pure check (field comparison, type
// switch, etc). In such cases splitting is just [Tee] plus an [OrderedFilter]
// on each branch:
//
//	adults, minors := rill.Tee(users)
//	adults = rill.OrderedFilter(adults, 1, func(u User) (bool, error) { return u.Age >= 18, nil })
//	minors = rill.OrderedFilter(minors, 1, func(u User) (bool, error) { return u.Age < 18, nil })
//
// If the predicate is expensive, stateful, or can fail, it must be evaluated
// once per item, before [Tee]: inline this function's implementation, which
// tags each item with the decision and routes on the tag. The same pattern
// extends to n-way splitting by tagging with an index or key instead of a bool.
func OrderedSplit2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	type Decision[A any] struct {
		Value    A
		Decision bool
	}

	tagged := OrderedMap(in, n, func(a A) (Decision[A], error) {
		d, err := f(a)
		return Decision[A]{Value: a, Decision: d}, err
	})

	tagged1, tagged2 := Tee(tagged)
	outTrue = OrderedFilterMap(tagged1, 1, func(d Decision[A]) (A, bool, error) { return d.Value, d.Decision, nil })
	outFalse = OrderedFilterMap(tagged2, 1, func(d Decision[A]) (A, bool, error) { return d.Value, !d.Decision, nil })

	return
}

// Tee returns two streams that are identical to the input stream (both errors and values).
// Both output streams must be consumed independently to avoid deadlocks.
//
// This is a non-blocking function that processes items in a single goroutine.
// See the package documentation for more information on non-blocking functions and error handling.
//
// If deep copying of values is needed, use [Map] on one or both outputs:
//
//	out1, out2 := rill.Tee(in)
//	out2 = rill.Map(out2, 1, func(x A) (A, error) {
//		return deepCopy(x), nil
//	})
func Tee[A any](in <-chan Try[A]) (<-chan Try[A], <-chan Try[A]) {
	if in == nil {
		return nil, nil
	}

	out1 := make(chan Try[A])
	out2 := make(chan Try[A])

	go func() {
		defer close(out1)
		defer close(out2)

		for x := range in {
			out1 <- x
			out2 <- x
		}
	}()

	return out1, out2
}

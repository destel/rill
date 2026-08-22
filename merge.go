package rill

import (
	"github.com/destel/rill/internal/core"
)

// Merge performs a fan-in, combining multiple channels into a single
// output channel. It returns immediately, and consumes the inputs
// simultaneously and independently, interleaving their items in the
// output as they arrive. Merge preserves the relative order of items
// from the same input.
//
// The output is closed only when all inputs are exhausted.
// A nil input is never exhausted, so the output never closes.
// Merge with no arguments immediately returns an empty closed channel.
func Merge[A any](ins ...<-chan A) <-chan A {
	return core.Merge(ins...)
}

// Split2 divides the stream into two streams: values that match the
// condition f go to outTrue, and the rest go to outFalse. Errors are
// sent to both outputs.
//
// The streams must be consumed concurrently to avoid a deadlock.
//
// The argument n bounds the number of concurrent calls to f. Items are
// written to the outputs as they become ready, so their order can
// differ from the input order when n > 1. Use [OrderedSplit2] to
// preserve the order.
//
// See the package documentation for the behaviors that all stages share.
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
	validateN(n)
	validateNilFunc(f == nil)

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

// OrderedSplit2 is the ordered version of [Split2]: the outputs
// preserve the input order, for values and errors alike.
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
	validateN(n)
	validateNilFunc(f == nil)

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

// Tee duplicates the input channel into two identical channels. It
// returns immediately, forwards each item to both outputs as it
// arrives, and closes both once the input is exhausted.
//
// The outputs must be consumed concurrently to avoid a deadlock.
//
// If deep copying of values is needed, use [Map] on one or both
// outputs:
//
//	out1, out2 := rill.Tee(in)
//	out2 = rill.Map(out2, 1, func(x A) (A, error) {
//		return deepCopy(x), nil
//	})
func Tee[A any](in <-chan A) (<-chan A, <-chan A) {
	if in == nil {
		return nil, nil
	}

	out1 := make(chan A)
	out2 := make(chan A)

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

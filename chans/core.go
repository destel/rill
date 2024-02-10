package chans

import "github.com/destel/rill/internal/common"

func Map[A, B any](in <-chan A, n int, f func(A) B) <-chan B {
	return common.MapOrFilter(in, n, func(a A) (B, bool) {
		return f(a), true
	})
}

func OrderedMap[A, B any](in <-chan A, n int, f func(A) B) <-chan B {
	return common.OrderedMapOrFilter(in, n, func(a A) (B, bool) {
		return f(a), true
	})
}

func Filter[A any](in <-chan A, n int, f func(A) bool) <-chan A {
	return common.MapOrFilter(in, n, func(a A) (A, bool) {
		return a, f(a)
	})
}

func OrderedFilter[A any](in <-chan A, n int, f func(A) bool) <-chan A {
	return common.OrderedMapOrFilter(in, n, func(a A) (A, bool) {
		return a, f(a)
	})
}

func FlatMap[A, B any](in <-chan A, n int, f func(A) <-chan B) <-chan B {
	var zero B
	return common.MapOrFlatMap(in, n, func(a A) (<-chan B, B) {
		return f(a), zero
	})
}

func OrderedFlatMap[A, B any](in <-chan A, n int, f func(A) <-chan B) <-chan B {
	var zero B
	return common.OrderedMapOrFlatMap(in, n, func(a A) (<-chan B, B) {
		return f(a), zero
	})
}

// blocking
// todo: explain that if false has been returned for item[i] that it's guranteed that function would have been called for all previous items
func ForEach[A any](in <-chan A, n int, f func(A) bool) {
	if n == 1 {
		for a := range in {
			if !f(a) {
				break
			}
		}

		return
	}

	in, earlyExit := common.Break(in)
	done := make(chan struct{})

	common.Loop(in, done, n, func(a A) {
		if !f(a) {
			earlyExit()
		}
	})

	<-done
}

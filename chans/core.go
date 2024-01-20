package chans

import (
	"sync"
)

func loop[A any](in <-chan A, n int, f func(A)) *sync.WaitGroup {
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for a := range in {
				f(a)
			}
			return
		}()
	}

	return &wg
}

func MapAndFilter[A, B any](in <-chan A, n int, f func(A) (B, bool)) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)

	wg := loop(in, n, func(x A) {
		y, keep := f(x)
		if keep {
			out <- y
		}
	})

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func Map[A, B any](in <-chan A, n int, f func(A) B) <-chan B {
	return MapAndFilter(in, n, func(a A) (B, bool) {
		return f(a), true
	})
}

func Filter[A any](in <-chan A, n int, f func(A) bool) <-chan A {
	return MapAndFilter(in, n, func(a A) (A, bool) {
		return a, f(a)
	})
}

// todo: comment about deadlocks
func FlatMap[A, B any](in <-chan A, n int, f func(A) <-chan B) <-chan B {
	out := make(chan B)

	wg := loop(in, n, func(a A) {
		for b := range f(a) {
			out <- b
		}
	})

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

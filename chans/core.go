package chans

import (
	"fmt"
	"sync"
	"sync/atomic"
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
	if in == nil {
		return nil
	}

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

// blocking
// todo: explain that if false has been returned for item[i] that it's guranteed that function would have been called for all previous items
func ForEach[A any](in <-chan A, n int, f func(A) bool) {
	// In case of early exit some unconsumed items will be left in the 'in' channel.
	// To avoid leaks we need to consume everything until channel is closed.
	// On the other hand caller can close in, only after we return.
	// So drain also must happen only after we return. The correct order is:
	// early exit -> caller closes 'in' -> drain 'in'
	// That's why we're using non-blocking drain here.
	defer DrainNB(in)

	if n == 1 {
		for a := range in {
			if !f(a) {
				break
			}
		}

		return
	}

	var wg sync.WaitGroup
	earlyExit := int64(0)

	for i := 0; i < n; i++ {
		j := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			for a := range in {
				fmt.Println("Goroutine", j, "got", a)
				ok := f(a)
				if !ok {
					atomic.AddInt64(&earlyExit, 1)
					break
				} else if atomic.LoadInt64(&earlyExit) > 0 {
					break
				}
			}
		}()
	}

	wg.Wait()
}

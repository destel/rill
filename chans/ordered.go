package chans

import (
	"sync"
)

type orderedValue[A any] struct {
	Value A
	Index int
}

// 'f' function must consist of three steps:
// 1. Calculate the output value. This step is executed immediately and concurrently.
// 2. waitMyTurn() must be called to ensure that the output values are sent in the correct order
// 3. Send/write the output value somewhere.
func orderedLoop[A any](in <-chan A, n int, f func(a A, waitMyTurn func())) *sync.WaitGroup {
	orderedIn := make(chan orderedValue[A])
	go func() {
		defer close(orderedIn)
		i := 0
		for x := range in {
			orderedIn <- orderedValue[A]{x, i}
			i++
		}
	}()

	nextIndex := 0

	mu := new(sync.Mutex)
	inProgress := make(map[int]*sync.Cond, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		cond := sync.NewCond(mu)

		wg.Add(1)
		go func() {
			defer wg.Done()

			for a := range orderedIn {
				waitMyTurn := func() {
					mu.Lock()
					inProgress[a.Index] = cond

					for nextIndex != a.Index {
						cond.Wait() // wait for our turn
					}
				}

				f(a.Value, waitMyTurn)

				nextIndex++
				if otherCond, ok := inProgress[nextIndex]; ok {
					otherCond.Signal()
				}

				delete(inProgress, a.Index)
				mu.Unlock()
			}
		}()
	}

	return &wg
}

func OrderedMapAndFilter[A, B any](in <-chan A, n int, f func(A) (B, bool)) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)
	wg := orderedLoop(in, n, func(a A, waitMyTurn func()) {
		y, keep := f(a)

		waitMyTurn()

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

func OrderedMap[A, B any](in <-chan A, n int, f func(A) B) <-chan B {
	return OrderedMapAndFilter(in, n, func(a A) (B, bool) {
		return f(a), true
	})
}

func OrderedFilter[A any](in <-chan A, n int, f func(A) bool) <-chan A {
	return OrderedMapAndFilter(in, n, func(a A) (A, bool) {
		return a, f(a)
	})
}

// todo: deadlocks comment
func OrderedFlatMap[A, B any](in <-chan A, n int, f func(A) <-chan B) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)
	wg := orderedLoop(in, n, func(a A, waitMyTurn func()) {
		bb := f(a)
		waitMyTurn()
		for b := range bb {
			out <- b
		}
	})

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

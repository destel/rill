package core

import (
	"sync"
)

func sequentialReduce[A any](in <-chan A, f func(A, A) A) (A, bool) {
	var res A
	var seen bool

	for a := range in {
		if seen {
			res = f(res, a)
		} else {
			res = a
			seen = true
		}
	}

	return res, seen
}

func reduce[A any](in <-chan A, n int, sema Semaphore, f func(A, A) A) (A, bool) {
	if n == 1 {
		return sequentialReduce(in, f)
	}

	results := make(chan A, n)

	var wg sync.WaitGroup

	for i := 0; i < n/2; i++ {
		wg.Add(1)
		sema.Acquire()
		go func() {
			defer sema.Release()
			defer wg.Done()

			res, ok := sequentialReduce(in, f)
			if ok {
				results <- res
			}

		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return reduce(results, n/2, sema, f)
}

func Reduce[A any](in <-chan A, n int, f func(A, A) A) (A, bool) {
	sema := make(Semaphore, n)
	return reduce(in, n, sema, f)
}

func MapReduce[A any, K comparable, V any](in <-chan A, nm int, mapFunc func(A) (K, V), nr int, reduceFunc func(V, V) V) map[K]V {
	// todo: need to think how mn and nr should work

	mapResults := make(chan map[K]V, nm)
	var wg sync.WaitGroup

	for i := 0; i < nm; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			localMap := make(map[K]V)

			for a := range in {
				k, v := mapFunc(a)
				if oldV, ok := localMap[k]; ok {
					localMap[k] = reduceFunc(oldV, v)
				} else {
					localMap[k] = v
				}
			}

			mapResults <- localMap
		}()
	}

	go func() {
		wg.Wait()
		close(mapResults)
	}()

	res, _ := Reduce(mapResults, nr, func(m1, m2 map[K]V) map[K]V {
		if len(m2) > len(m1) {
			m1, m2 = m2, m1
		}

		for k, v := range m2 {
			if oldV, ok := m1[k]; ok {
				m1[k] = reduceFunc(oldV, v)
			} else {
				m1[k] = v
			}
		}

		return m1
	})

	return res
}

package core

import (
	"sync"
)

// nonConcurrentReduce is a non-concurrent version of Reduce.
func nonConcurrentReduce[A any](in <-chan A, reducer func(A, A) A) (A, bool) {
	res, ok := <-in
	if !ok {
		return res, false
	}

	for a := range in {
		res = reducer(res, a)
	}

	return res, true
}

// reduce reduces the input channel by applying the reducer function to pairs of elements.
// Total number of spawned goroutines is between n and 2n
// It also takes a semaphore for precise control over the number of concurrent goroutines.
func reduce[A any](in <-chan A, sema Semaphore, n int, reducer func(A, A) A) (A, bool) {
	// Phase 0: Optimized non-concurrent case
	if n == 1 {
		return nonConcurrentReduce(in, reducer)
	}

	// Phase 1: Each goroutine calculates its own partial result
	partialResults := make(chan A, n)
	var wg sync.WaitGroup

	for i := 0; i < n/2; i++ {
		wg.Add(1)
		sema.Acquire()
		go func() {
			defer sema.Release()
			defer wg.Done()

			res, ok := nonConcurrentReduce(in, reducer)
			if ok {
				partialResults <- res
			}
		}()
	}

	go func() {
		wg.Wait()
		close(partialResults)
	}()

	// Phase 2: Recursive call. Reduce partial partialResults into a single value.
	// partialResults channel contains at most n elements, which implies we can have at most n/2 concurrent reductions.
	// Total number of spawned goroutines therefore is n + n/2 + n/4 + ... = 2n. But because of integer division, it's actually less than 2n.
	// Recursion depth is at most log2(n)
	// Overall number of goroutines and recursion depth are independent of the input size.
	return reduce(partialResults, sema, n/2, reducer)
}

// todo: document
func Reduce[A any](in <-chan A, n int, f func(A, A) A) (A, bool) {
	return reduce(in, make(Semaphore, n), n, f)
}

type keyValue[K, V any] struct {
	Key   K
	Value V
}

// reduceIntoMap is a helper function that adds a new key-value pair to the map or reduces the value of an existing key.
func reduceIntoMap[K comparable, V any](m map[K]V, k K, v V, reducer func(V, V) V) {
	if oldV, ok := m[k]; ok {
		m[k] = reducer(oldV, v)
	} else {
		m[k] = v
	}
}

// todo: document
func MapReduce[A any, K comparable, V any](in <-chan A, nm int, mapper func(A) (K, V), nr int, reducer func(V, V) V) map[K]V {
	// Phase 0: Optimized non-concurrent case
	if nm == 1 && nr == 1 {
		res := make(map[K]V)
		for a := range in {
			k, v := mapper(a)
			reduceIntoMap(res, k, v, reducer)
		}
		return res
	}

	// Phase 1: Map
	mapped := MapOrFilter(in, nm, func(a A) (keyValue[K, V], bool) {
		k, v := mapper(a)
		return keyValue[K, V]{k, v}, true
	})

	// Phase 2.1: Optimized non-concurrent reduce. Build a final map right away.
	if nr == 1 {
		res := make(map[K]V)
		for kv := range mapped {
			reduceIntoMap(res, kv.Key, kv.Value, reducer)
		}
		return res
	}

	// Phase 2.2: Each goroutine builds its own partial map
	partialResults := make(chan map[K]V, nr)
	sema := make(Semaphore, nr)
	var wg sync.WaitGroup

	for i := 0; i < nr; i++ {
		wg.Add(1)
		sema.Acquire()
		go func() {
			defer sema.Release()
			defer wg.Done()

			res := make(map[K]V)
			for kv := range mapped {
				reduceIntoMap(res, kv.Key, kv.Value, reducer)
			}
			partialResults <- res
		}()
	}

	go func() {
		wg.Wait()
		close(partialResults)
	}()

	// Phase 3: Merge all partial maps into a single one
	res, _ := reduce(partialResults, sema, nr/2, func(m1, m2 map[K]V) map[K]V {
		// Always merge smaller map into a bigger one
		if len(m2) > len(m1) {
			m1, m2 = m2, m1
		}

		for k, v := range m2 {
			reduceIntoMap(m1, k, v, reducer)
		}
		return m1
	})

	return res
}

package core

import (
	"sync"
)

// nonConcurrentReduce is a non-concurrent version of Reduce.
func nonConcurrentReduce[A any](in <-chan A, f func(A, A) A) (A, bool) {
	res, ok := <-in
	if !ok {
		return res, false
	}

	for a := range in {
		res = f(res, a)
	}

	return res, true
}

// todo: document
func Reduce[A any](in <-chan A, n int, f func(A, A) A) (A, bool) {
	// Phase 0: Optimized non-concurrent case
	if n == 1 {
		return nonConcurrentReduce(in, f)
	}

	// Phase 1: Each goroutine calculates its own partial result
	partialResults := make(chan A, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			res, ok := nonConcurrentReduce(in, f)
			if ok {
				partialResults <- res
			}
		}()
	}

	go func() {
		wg.Wait()
		close(partialResults)
	}()

	// Phase 2: Recursive call. Reduce partialResults into a single value.
	// Both the number of goroutines and the recursion depth are independent of the input size.
	//
	// The partialResults channel contains at most n elements, which will be grouped into at most n/2 pairs in the next recursion level.
	// The total number of concurrent goroutines is n + n/2 + n/4 + ... = 2n. However, due to integer division, it's actually less than 2n.
	// The number of concurrent reductions at any given moment is at most n (see below).
	// The recursion depth is at most log2(n).
	//
	// Number of concurrent reductions:
	// - At the current level, there are at most n concurrent reductions.
	// - For each additional concurrent reduction at the next level, at least two goroutines from the current level need to
	//   finish and send their partial results through the partialResults channel.
	// - This implies that when the number of concurrent reductions increases by 1 at the next level, it decreases by at least 2 at the current level.
	// - Consequently, the total number of concurrent reductions across all levels starts from n and decreases as data travels down the stack.
	return Reduce(partialResults, n/2, f)
}

type keyValue[K, V any] struct {
	Key   K
	Value V
}

// reduceIntoMap is a helper function that adds a new key-value pair to the map or reduces the value of an existing key.
func reduceIntoMap[K comparable, V any](m map[K]V, k K, v V, f func(V, V) V) {
	if oldV, ok := m[k]; ok {
		m[k] = f(oldV, v)
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
	var wg sync.WaitGroup

	for i := 0; i < nr; i++ {
		wg.Add(1)
		go func() {
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
	res, _ := Reduce(partialResults, nr/2, func(m1, m2 map[K]V) map[K]V {
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

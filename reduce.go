package rill

import (
	"sync"
	"sync/atomic"

	"github.com/destel/rill/internal/core"
	"github.com/destel/rill/internal/list"
)

// Reduce combines all items from the input stream into a single value using a binary function f.
//
// Treating f as a binary operator "*", Reduce computes in[0] * in[1] * ... * in[N-1]:
// items are combined in stream order, but the parenthesization is non-deterministic
// and may vary from run to run. This requires f to be associative -
// (a * b) * c == a * (b * c) - so that every parenthesization yields the same
// result. Commutativity is not required.
//
// The hasResult return flag is set to true if the stream contained at least one value and no error was encountered,
// otherwise it is set to false.
//
// Reduce is a blocking function that processes items concurrently using n goroutines.
//
// See the package documentation for more information on blocking functions and error handling.
func Reduce[A any](in <-chan Try[A], n int, f func(A, A) (A, error)) (result A, hasResult bool, err error) {
	validateN(n)
	validateNilFunc(f == nil)

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
			var zero A
			return zero, false, err
		}
		return acc, seeded, nil
	}

	// The high level idea: keep values in stream order in a linked list.
	// A worker takes two adjacent nodes and merges them into one.
	// If there's nothing to merge, worker pulls more values and appends them to the list.
	// If there is nothing left to pull, worker quits.
	//
	// This gives us:
	// - Reduction function can be non-commutative. Associativity is still required.
	// - Maximized utilization: a worker quits only when no current or future work is available for it.
	// - Convergence: after all workers quit, the list contains 0 or 1 nodes.
	// - With n workers, at most 2*n+1 nodes are live at any time.
	// - The reduction tree is adaptive. It depends on both the scheduler and the observed cost of f.
	//   If one partial result is expensive to produce, other workers keep merging around it,
	//   so the emergent tree tends to be near-balanced exactly when it matters.

	type Item struct {
		value A
		owned bool
	}

	// Pool of list nodes to avoid per-item allocations.
	pool := &core.Pool[*list.Node[Item]]{
		New:            func() *list.Node[Item] { return new(list.Node[Item]) },
		Reset:          func(node *list.Node[Item]) { node.Value = Item{} },
		Unsynchronized: true,
	}

	nodes := list.New[Item]()

	// mu protects the state (list, pool, node values),
	// inputMu ensures that pulling from the input channel and appending to the list is atomic
	var mu sync.Mutex
	var inputMu core.DurableMutex

	// errors and the final result are reported via the out channel
	var errSeen atomic.Bool
	out := make(chan Try[A], n)

	reportError := func(err error) {
		errSeen.Store(true)
		out <- Try[A]{Error: err}
	}

	// Pulls at most 2 values from the input channel and appends them to the list.
	// The first pulled value is marked as owned.
	pull2 := func() *list.Node[Item] {
		inputMu.Lock()
		defer inputMu.Unlock()

		var buf [2]A
		count := 0

		for a := range in {
			if errSeen.Load() {
				return nil
			}
			if a.Error != nil {
				reportError(a.Error)
				return nil
			}
			buf[count] = a.Value
			count++
			if count == 2 {
				break
			}
		}

		if count == 0 {
			return nil
		}

		mu.Lock()
		defer mu.Unlock()

		first := pool.Get()
		first.Value = Item{value: buf[0], owned: true}
		nodes.PushBack(first)

		for i := 1; i < count; i++ {
			node := pool.Get()
			node.Value = Item{value: buf[i], owned: false}
			nodes.PushBack(node)
		}

		return first
	}

	// How a worker finds work without scanning the entire list:
	// Each worker owns some node L and on every iteration it checks if one of its neighbors is free
	// and absorbs it into L if so. Otherwise it pulls or quits.
	//
	// Quitting is optimal: any additional work discoverable by scanning would require
	// two adjacent free nodes. pull2 and release semantics make that impossible.
	worker := func() {
		var current *list.Node[Item]

		for {
			// As soon as any worker reported an error, all workers quit
			if errSeen.Load() {
				return
			}

			if current == nil {
				current = pull2()
				if current == nil {
					return
				}
			}

			mu.Lock()

			// Prefer the right neighbor so a fresh seat immediately
			// consumes its pair-mate. Either direction preserves operand
			// order.
			var x, y A
			var nodeToAbsorb *list.Node[Item]

			if right := current.Next(); right != nil && !right.Value.owned {
				x, y = current.Value.value, right.Value.value
				nodeToAbsorb = right
			} else if left := current.Prev(); left != nil && !left.Value.owned {
				x, y = left.Value.value, current.Value.value
				nodeToAbsorb = left
			}

			// Neither neighbor is free: release the current node and get back to pulling
			if nodeToAbsorb == nil {
				current.Value.owned = false
				current = nil
				mu.Unlock()
				continue
			}

			nodes.Remove(nodeToAbsorb)
			pool.Put(nodeToAbsorb)

			mu.Unlock()

			// Release the mutex before calling f
			merged, err := f(x, y)
			if err != nil {
				reportError(err)

				// The merge is abandoned, the caller may assume there are no
				// references left to x and y: remove the node from the list
				mu.Lock()
				nodes.Remove(current)
				pool.Put(current)
				mu.Unlock()
				return
			}

			mu.Lock()
			current.Value.value = merged
			mu.Unlock()
		}
	}

	defer Discard(in)

	// Start the workers
	var wg sync.WaitGroup
	for range n {
		wg.Go(worker)
	}

	// Wait until all workers quit
	go func() {
		wg.Wait()

		// By construction, the list contains 0 or 1 nodes
		if first := nodes.Front(); !errSeen.Load() && first != nil {
			out <- Try[A]{Value: first.Value.value}
		}
		close(out)
	}()

	return First(out)
}

// MapReduce transforms the input stream into a Go map using mapper and reducer functions.
// The transformation is performed in two concurrent phases.
//
//   - The mapper function transforms each input item into a key-value pair.
//   - The reducer function reduces values of the same key into a single value.
//     This phase has the same semantics as the [Reduce] function: for each key,
//     values are combined in stream order, but the parenthesization is non-deterministic,
//     so the reducer must be associative.
//
// An empty input stream produces an empty map.
//
// MapReduce is a blocking function that processes items concurrently using nm and nr goroutines
// for the mapper and reducer functions respectively.
//
// See the package documentation for more information on blocking functions and error handling.
func MapReduce[A any, K comparable, V any](in <-chan Try[A], nm int, mapper func(A) (K, V, error), nr int, reducer func(V, V) (V, error)) (map[K]V, error) {
	validateN(nm)
	validateNilFunc(mapper == nil)
	validateN(nr)
	validateNilFunc(reducer == nil)

	if nm == 1 && nr == 1 {
		acc := make(map[K]V)
		err := ForEach(in, 1, func(a A) error {
			k, v, err := mapper(a)
			if err != nil {
				return err
			}
			return upsertIntoMap(acc, k, v, reducer)
		})

		if err != nil {
			return nil, err
		}
		return acc, nil
	}

	defer Discard(in)
	var done atomic.Bool
	defer done.Store(true)

	// Pool for reusing intermediate maps.
	// The size is O(nm + nr).
	pool := &core.Pool[map[K]V]{
		New:   func() map[K]V { return make(map[K]V) },
		Reset: func(m map[K]V) { clear(m) },
	}

	// Turn each item into a single-key map
	singletons := OrderedFilterMap(in, nm, func(a A) (map[K]V, bool, error) {
		if done.Load() {
			return nil, false, nil
		}

		k, v, err := mapper(a)
		if err != nil {
			return nil, false, err
		}

		m := pool.Get()
		m[k] = v
		return m, true, nil
	})

	// Reduce the singletons into one final map
	res, ok, err := Reduce(singletons, nr, func(acc, m map[K]V) (map[K]V, error) {
		merged, leftover, err := mergeMaps(acc, m, reducer)
		pool.Put(leftover)
		return merged, err
	})

	if err != nil {
		return nil, err
	}
	if !ok {
		return make(map[K]V), nil
	}
	return res, nil
}

// upsertIntoMap adds the (k, v) pair to the map m.
// If the key already exists, it is merged with the new value using the merge function.
// On merge failure the map is left unchanged and the error is returned.
func upsertIntoMap[K comparable, V any](m map[K]V, k K, v V, mergeFunc func(V, V) (V, error)) error {
	old, ok := m[k]
	if !ok {
		m[k] = v
		return nil
	}

	newV, err := mergeFunc(old, v)
	if err != nil {
		return err
	}
	m[k] = newV
	return nil
}

// mergeMaps merges all keys from m into acc using upsertIntoMap and a reducer function.
// Merge is done in-place: the larger map is used as the storage for the final result,
// and the smaller one is returned as a leftover.
func mergeMaps[K comparable, V any](acc, m map[K]V, reducer func(V, V) (V, error)) (result, leftover map[K]V, err error) {
	if len(acc) >= len(m) {
		for k, v := range m {
			if err := upsertIntoMap(acc, k, v, reducer); err != nil {
				return acc, m, err
			}
		}
		return acc, m, nil
	}

	// acc is the smaller one, so pour it into m; flip the operands to keep
	// acc's value on the left.
	flipped := func(mV, accV V) (V, error) { return reducer(accV, mV) }
	for k, v := range acc {
		if err := upsertIntoMap(m, k, v, flipped); err != nil {
			return m, acc, err
		}
	}
	return m, acc, nil
}

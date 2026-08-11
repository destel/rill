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
	// - Convergence: after all workers quit, the list contains 0 or 1 nodes - at the last
	//   release, every other node was already free, so a second node would have been
	//   a free neighbor, and workers never release next to one.
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

	// mu protects the state (list, pool, node values).
	// inputMu ensures that pulling from the input channel and appending to the list is atomic
	var mu sync.Mutex
	var inputMu core.DurableMutex

	// errors and the final result are reported via the out channel
	var errSeen atomic.Bool
	out := make(chan Try[A], 1)

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
	// Workers are greedy: a worker never releases its node while a free neighbor
	// exists, so no two free nodes are ever adjacent. Quitting is optimal:
	// any work discoverable by scanning would be an adjacent free pair.
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

			nodeToAbsorb.Detach()
			pool.Put(nodeToAbsorb)

			mu.Unlock()

			// Release the mutex before calling f
			merged, err := f(x, y)
			if err != nil {
				reportError(err)

				// The merge is abandoned, the caller may assume there are no
				// references left to x and y: remove the node from the list
				mu.Lock()
				current.Detach()
				pool.Put(current)
				mu.Unlock()
				return
			}

			// no mutex needed: the node is owned by the worker
			current.Value.value = merged
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

			old, ok := acc[k]
			if !ok {
				acc[k] = v
				return nil
			}

			merged, err := reducer(old, v)
			if err != nil {
				return err
			}
			acc[k] = merged
			return nil
		})

		if err != nil {
			return nil, err
		}
		return acc, nil
	}

	// The high level idea: almost the same engine as in Reduce.
	//   - One linked list per key instead of the global one. Lists are created lazily
	//     as new keys are encountered.
	//   - Worker pulls and appends one value at a time instead of two.
	//   - Workers are not permanently bound to lists. When worker pulls a value, it appends it
	//     to that key's list and works there next.
	//   - O(c + n) live nodes for cardinality c.
	//   - Each individual list conforms to the same invariant as the global list
	//     in Reduce, and the algorithm converges to each list holding exactly one
	//     node: unlike the global list, a key's list is never empty - it is
	//     created by an append and never loses its last node.

	defer Discard(in)
	var done atomic.Bool
	defer done.Store(true)

	type pair struct {
		key   K
		value V
	}

	// Turn each item into a (key, value) pair
	pairs := OrderedFilterMap(in, nm, func(a A) (pair, bool, error) {
		if done.Load() {
			return pair{}, false, nil
		}

		k, v, err := mapper(a)
		if err != nil {
			return pair{}, false, err
		}
		return pair{key: k, value: v}, true, nil
	})
	defer Discard(pairs)

	type Item struct {
		value V
		owned bool
	}

	// Pool of list nodes, shared by all per-key lists.
	pool := &core.Pool[*list.Node[Item]]{
		New:            func() *list.Node[Item] { return new(list.Node[Item]) },
		Reset:          func(node *list.Node[Item]) { node.Value = Item{} },
		Unsynchronized: true,
	}

	lists := make(map[K]*list.List[Item])

	// mu protects the state (lists, pool, node values).
	// inputMu ensures that pulling from the input channel and appending to a list is atomic
	var mu sync.Mutex
	var inputMu core.DurableMutex

	// errors and the final map are reported via the out channel
	var errSeen atomic.Bool
	out := make(chan Try[map[K]V], 1)

	reportError := func(err error) {
		errSeen.Store(true)
		out <- Try[map[K]V]{Error: err}
	}

	// Pulls one pair from the input and appends it to its key's list.
	// The caller becomes the owner of the appended node.
	pull1 := func() *list.Node[Item] {
		inputMu.Lock()
		defer inputMu.Unlock()

		a, ok := <-pairs
		if !ok || errSeen.Load() {
			return nil
		}
		if a.Error != nil {
			reportError(a.Error)
			return nil
		}

		mu.Lock()
		defer mu.Unlock()

		l := lists[a.Value.key]
		if l == nil {
			l = list.New[Item]()
			lists[a.Value.key] = l
		}

		node := pool.Get()
		node.Value = Item{value: a.Value.value, owned: true}
		l.PushBack(node)

		return node
	}

	worker := func() {
		var current *list.Node[Item]

		for {
			// As soon as any worker reported an error, all workers quit
			if errSeen.Load() {
				return
			}

			if current == nil {
				current = pull1()
				if current == nil {
					return
				}
			}

			mu.Lock()

			// A fresh node sits at the back of its list, so left neighbor is checked first.
			// Either direction preserves operand order.
			var x, y V
			var nodeToAbsorb *list.Node[Item]

			if left := current.Prev(); left != nil && !left.Value.owned {
				x, y = left.Value.value, current.Value.value
				nodeToAbsorb = left
			} else if right := current.Next(); right != nil && !right.Value.owned {
				x, y = current.Value.value, right.Value.value
				nodeToAbsorb = right
			}

			// Neither neighbor is free: release the current node and get back to pulling
			if nodeToAbsorb == nil {
				current.Value.owned = false
				current = nil
				mu.Unlock()
				continue
			}

			nodeToAbsorb.Detach()
			pool.Put(nodeToAbsorb)

			mu.Unlock()

			// Release the mutex before calling the reducer
			merged, err := reducer(x, y)
			if err != nil {
				reportError(err)

				// The merge is abandoned: remove the node from its list
				mu.Lock()
				current.Detach()
				pool.Put(current)
				mu.Unlock()
				return
			}

			// no mutex needed: the node is owned by the worker
			current.Value.value = merged
		}
	}

	// Start the workers
	var wg sync.WaitGroup
	for range nr {
		wg.Go(worker)
	}

	// Wait until all workers quit, then harvest the final map
	go func() {
		wg.Wait()

		if !errSeen.Load() {
			res := make(map[K]V, len(lists))
			// By construction, each list has converged to exactly one node
			for k, l := range lists {
				res[k] = l.Front().Value.value
			}
			out <- Try[map[K]V]{Value: res}
		}
		close(out)
	}()

	res, _, err := First(out)
	if err != nil {
		return nil, err
	}
	return res, nil
}

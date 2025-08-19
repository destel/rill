package heapbuffer

// Heap provides a generic min-heap implementation.
// The implementation is inspired by and follows similar patterns to Go's container/heap
// (https://golang.org/src/container/heap/heap.go) but uses generics instead of interfaces
// and adds features like OnIndexChange callback for index tracking.
// Core heap operations (up/down, remove, insert) follow standard textbook algorithms.
type Heap[T any] struct {
	data []T

	less          func(item1, item2 T) bool
	OnIndexChange func(T, int)
}

func NewHeap[T any](less func(item1, item2 T) bool) *Heap[T] {
	return &Heap[T]{less: less}
}

func (h *Heap[T]) SetData(data []T) {
	h.data = data

	// heapify
	n := h.Len()
	for i := n/2 - 1; i >= 0; i-- {
		h.down(i, n)
	}
}

func (h *Heap[T]) Len() int {
	return len(h.data)
}

// Grow grows the heap's capacity, if necessary, to guarantee space for another n items.
func (h *Heap[T]) Grow(n int) {
	targetCap := len(h.data) + n
	if targetCap > cap(h.data) {
		data := make([]T, len(h.data), targetCap)
		copy(data, h.data)
		h.data = data
	}
}

func (h *Heap[T]) Push(item T) {
	h.data = append(h.data, item)
	n := h.Len() - 1
	if h.OnIndexChange != nil {
		h.OnIndexChange(item, n)
	}
	h.up(n)
}

func (h *Heap[T]) Peek() T {
	return h.data[0]
}

func (h *Heap[T]) Pop() T {
	n := h.Len() - 1
	h.swap(0, n)
	h.down(0, n)
	return h.removeLast()
}

func (h *Heap[T]) Remove(i int) T {
	n := h.Len() - 1
	if i != n {
		h.swap(i, n)
		if !h.down(i, n) {
			h.up(i)
		}
	}

	return h.removeLast()
}

func (h *Heap[T]) Fix(i int) {
	if !h.down(i, h.Len()) {
		h.up(i)
	}
}

func (h *Heap[T]) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !h.less(h.data[j], h.data[i]) {
			break
		}

		h.swap(i, j)
		j = i
	}
}

func (h *Heap[T]) down(i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && h.less(h.data[j2], h.data[j1]) {
			j = j2 // = 2*i + 2  // right child
		}
		if !h.less(h.data[j], h.data[i]) {
			break
		}

		h.swap(i, j)
		i = j
	}
	return i > i0
}

func (h *Heap[T]) swap(i, j int) {
	h.data[i], h.data[j] = h.data[j], h.data[i]

	if h.OnIndexChange != nil {
		// This is the place where we become a bit less optimal than the container/heap
		// We do 2 function calls while container/heap does 1
		h.OnIndexChange(h.data[i], i)
		h.OnIndexChange(h.data[j], j)
	}
}

func (h *Heap[T]) removeLast() T {
	var zero T

	n := h.Len() - 1
	res := h.data[n]
	h.data[n] = zero // for GC
	h.data = h.data[:n]

	if h.OnIndexChange != nil {
		h.OnIndexChange(res, -1) // for safety
	}
	return res
}

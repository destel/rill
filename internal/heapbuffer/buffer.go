package heapbuffer

type Buffer[T any] struct {
	heap     *Heap[T]
	capacity int
}

// New creates a priority queue buffer with the given capacity and less function.
// Non-zero capacity will create a fixed size buffer. Any write operation on a full buffer will panic.
func New[T any](capacity int, less func(item1, item2 T) bool) *Buffer[T] {
	h := NewHeap[T](less)
	if capacity > 0 {
		h.Grow(capacity)
	}

	return &Buffer[T]{
		heap:     h,
		capacity: capacity,
	}
}

func (b *Buffer[T]) IsEmpty() bool {
	return b.heap.Len() == 0
}

func (b *Buffer[T]) IsFull() bool {
	if b.capacity <= 0 {
		return false
	}

	return b.heap.Len() >= b.capacity
}

func (b *Buffer[T]) Peek() T {
	return b.heap.Peek()
}

func (b *Buffer[T]) Read() T {
	return b.heap.Pop()
}

func (b *Buffer[T]) Write(v T) {
	if b.IsFull() {
		panic("pqbuffer: buffer is full")
	}
	b.heap.Push(v)
}

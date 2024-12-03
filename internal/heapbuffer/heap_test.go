package heapbuffer

import (
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestHeapSimple(t *testing.T) {
	h := NewHeap(func(item1, item2 int) bool {
		return item1 < item2
	})

	th.ExpectValue(t, h.Len(), 0)

	h.Push(2)
	h.Push(6)
	h.Push(1)
	h.Push(2)
	h.Push(4)
	h.Push(2)

	th.ExpectValue(t, h.Len(), 6)
	th.ExpectValue(t, h.Peek(), 1)

	th.ExpectValue(t, h.Pop(), 1)
	th.ExpectValue(t, h.Pop(), 2)
	th.ExpectValue(t, h.Pop(), 2)
	th.ExpectValue(t, h.Pop(), 2)
	th.ExpectValue(t, h.Pop(), 4)
	th.ExpectValue(t, h.Pop(), 6)

	th.ExpectPanic(t, func() {
		h.Pop()
	})
	th.ExpectPanic(t, func() {
		h.Peek()
	})

	th.ExpectValue(t, h.Len(), 0)

	h.SetData([]int{3, 1, 2})

	th.ExpectValue(t, h.Len(), 3)
	th.ExpectValue(t, h.Pop(), 1)
	th.ExpectValue(t, h.Pop(), 2)
	th.ExpectValue(t, h.Pop(), 3)
}

type pqItem struct {
	value     string
	priority  int
	heapIndex int
}

func TestPriorityQueue(t *testing.T) {
	h := NewHeap(func(item1, item2 *pqItem) bool {
		return item1.priority < item2.priority
	})

	h.OnIndexChange = func(item *pqItem, i int) {
		item.heapIndex = i
	}

	item1 := &pqItem{value: "item1", priority: 4}
	item2 := &pqItem{value: "item2", priority: 3}
	item3 := &pqItem{value: "item3", priority: 1}
	item4 := &pqItem{value: "item4", priority: 2}

	h.Push(item1)
	h.Push(item2)
	h.Push(item3)
	h.Push(item4)

	th.ExpectValue(t, h.Pop(), item3)
	th.ExpectValue(t, h.Pop(), item4)
	th.ExpectValue(t, h.Pop(), item2)
	th.ExpectValue(t, h.Pop(), item1)

	h.Push(item1)
	h.Push(item2)
	h.Push(item3)
	h.Push(item4)

	item4.priority = 5
	h.Fix(item4.heapIndex)

	item3.priority = 6
	h.Fix(item3.heapIndex)

	item1.priority = 1
	h.Fix(item1.heapIndex)

	h.Remove(item2.heapIndex)

	th.ExpectValue(t, h.Pop(), item1)
	th.ExpectValue(t, h.Pop(), item4)
	th.ExpectValue(t, h.Pop(), item3)
}

func TestPriorityQueueIndices(t *testing.T) {
	h := NewHeap(func(item1, item2 *pqItem) bool {
		return item1.priority < item2.priority
	})

	h.OnIndexChange = func(item *pqItem, i int) {
		item.heapIndex = i
	}

	item1 := &pqItem{value: "item1", priority: 1}
	item2 := &pqItem{value: "item2", priority: 1}
	item3 := &pqItem{value: "item3", priority: 1}
	item4 := &pqItem{value: "item4", priority: 1}

	h.Push(item1)
	h.Push(item2)
	h.Push(item3)
	h.Push(item4)

	// delete in random order
	h.Remove(item3.heapIndex)
	h.Remove(item1.heapIndex)
	h.Remove(item4.heapIndex)
	h.Remove(item2.heapIndex)

	th.ExpectValue(t, h.Len(), 0)
}

func TestHeapGrow(t *testing.T) {
	h := NewHeap(func(item1, item2 int) bool {
		return item1 < item2
	})

	th.ExpectValue(t, cap(h.data)-len(h.data), 0)

	h.Grow(10)

	th.ExpectValue(t, cap(h.data)-len(h.data), 10)
}

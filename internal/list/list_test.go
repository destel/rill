package list

import (
	"slices"
	"testing"

	"github.com/destel/rill/internal/th"
)

func node(v int) *Node[int] { return &Node[int]{Value: v} }

// expectList checks contents via forward and backward iteration.
func expectList(t *testing.T, l *List[int], want ...int) {
	t.Helper()

	var forward []int
	for e := l.Front(); e != nil; e = e.Next() {
		forward = append(forward, e.Value)
		th.ExpectValue(t, e.list, l)
	}
	th.ExpectSlice(t, forward, want)
	if t.Failed() {
		return
	}

	var backward []int
	for e := l.Back(); e != nil; e = e.Prev() {
		backward = append(backward, e.Value)
	}
	slices.Reverse(backward)
	th.ExpectSlice(t, backward, want)
}

func TestEmpty(t *testing.T) {
	l := New[int]()
	expectList(t, l)
	th.ExpectValue(t, l.Front(), nil)
	th.ExpectValue(t, l.Back(), nil)

}

func TestPush(t *testing.T) {
	l := New[int]()
	e1, e2, e3 := node(1), node(2), node(3)

	l.PushBack(e1) // first op on a fresh list
	l.PushBack(e2)
	l.PushFront(e3)
	expectList(t, l, 3, 1, 2)

	l.PushBack(e3) // first to back
	expectList(t, l, 1, 2, 3)

	l.PushBack(e3) // already at back
	expectList(t, l, 1, 2, 3)

	l.PushFront(e3) // last to front
	expectList(t, l, 3, 1, 2)
}

func TestInsert(t *testing.T) {
	l := New[int]()
	e1, e2, e3 := node(1), node(2), node(3)

	l.PushBack(e1)
	l.PushBack(e2)

	l.InsertAfter(e3, e1) // detached into the middle
	expectList(t, l, 1, 3, 2)

	l.InsertAfter(e1, e2) // first to back
	expectList(t, l, 3, 2, 1)

	l.InsertAfter(e1, e2) // already there
	expectList(t, l, 3, 2, 1)

	l.InsertAfter(e2, e2) // n == mark
	expectList(t, l, 3, 2, 1)

	l.InsertBefore(e2, e3) // last to front
	expectList(t, l, 2, 3, 1)

	l.InsertBefore(e2, e3) // already before mark
	expectList(t, l, 2, 3, 1)

	l.InsertBefore(node(4), e2) // detached before the first node
	expectList(t, l, 4, 2, 3, 1)
}

func TestDetach(t *testing.T) {
	l := New[int]()
	e1, e2, e3 := node(1), node(2), node(3)
	l.PushBack(e1)
	l.PushBack(e2)
	l.PushBack(e3)

	e2.Detach()
	expectList(t, l, 1, 3)
	th.ExpectValue(t, e2.Next(), nil)
	th.ExpectValue(t, e2.Prev(), nil)
	th.ExpectValue(t, e2.list, nil)

	e2.Detach() // already detached
	expectList(t, l, 1, 3)

	e1.Detach()
	e3.Detach()
	expectList(t, l)
}

func TestCrossList(t *testing.T) {
	l1, l2 := New[int](), New[int]()
	e1, e2 := node(1), node(2)
	l1.PushBack(e1)
	l1.PushBack(e2)

	// n belongs to another list: no-ops
	l2.PushBack(e1)
	l2.PushFront(e1)
	expectList(t, l1, 1, 2)
	expectList(t, l2)

	// mark belongs to another list: no-op
	e3 := node(3)
	l2.InsertAfter(e3, e1)
	l2.InsertBefore(e3, e1)
	expectList(t, l2)

	// detach first, then move across
	e1.Detach()
	l2.PushBack(e1)
	expectList(t, l1, 2)
	expectList(t, l2, 1)
}

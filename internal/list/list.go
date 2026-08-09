// Package list provides a doubly linked list built around reusable,
// caller-owned nodes.
package list

// Node is a node of a List. The zero value is a detached node ready to be
// inserted into a list; removal detaches it again, so a node can be reused -
// including in a different list.
type Node[T any] struct {
	Value T

	next, prev *Node[T]
	list       *List[T]
}

// Next returns the next node, or nil if n is the last one or detached.
func (n *Node[T]) Next() *Node[T] {
	if n.next == nil || n.next.next == nil {
		return nil
	}
	return n.next
}

// Prev returns the previous node, or nil if n is the first one or detached.
func (n *Node[T]) Prev() *Node[T] {
	if n.prev == nil || n.prev.prev == nil {
		return nil
	}
	return n.prev
}

// List is a doubly linked list. Lists must be created with [New].
type List[T any] struct {
	front, back Node[T]
}

// New returns an initialized empty list.
func New[T any]() *List[T] {
	l := &List[T]{}
	l.front.next = &l.back
	l.back.prev = &l.front
	return l
}

// Front returns the first node, or nil if the list is empty.
func (l *List[T]) Front() *Node[T] {
	if l.front.next == &l.back {
		return nil
	}
	return l.front.next
}

// Back returns the last node, or nil if the list is empty.
func (l *List[T]) Back() *Node[T] {
	if l.front.next == &l.back {
		return nil
	}
	return l.back.prev
}

// Remove detaches n from the list. It is a no-op if n is not a node of l.
func (l *List[T]) Remove(n *Node[T]) {
	if n.list != l {
		return
	}

	n.prev.next = n.next
	n.next.prev = n.prev
	n.next = nil
	n.prev = nil
	n.list = nil
}

// PushBack inserts n at the back of the list. n can be a detached node or a
// node of l, in which case it is moved to the new position. Pushing a node of
// another list is a no-op - remove it there first.
func (l *List[T]) PushBack(n *Node[T]) {
	l.insertAfter(n, l.back.prev)
}

// PushFront inserts n at the front of the list. Same contract for n as
// [List.PushBack].
func (l *List[T]) PushFront(n *Node[T]) {
	l.insertAfter(n, &l.front)
}

// InsertAfter inserts n after mark, which must be a node of l. Same contract
// for n as [List.PushBack].
func (l *List[T]) InsertAfter(n, mark *Node[T]) {
	if mark.list == l {
		l.insertAfter(n, mark)
	}
}

// InsertBefore inserts n before mark. Same contract as [List.InsertAfter].
func (l *List[T]) InsertBefore(n, mark *Node[T]) {
	if mark.list == l {
		l.insertAfter(n, mark.prev)
	}
}

// mark can be the front sentinel
func (l *List[T]) insertAfter(n, mark *Node[T]) {
	if mark == n {
		return
	}
	if n.list == l {
		n.prev.next = n.next
		n.next.prev = n.prev
	} else if n.list != nil {
		return
	}

	next := mark.next
	mark.next = n
	n.prev = mark
	n.next = next
	next.prev = n
	n.list = l
}

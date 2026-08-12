// Package list provides a doubly linked list built around reusable,
// caller-owned nodes.
package list

// Node is a node of a List. The zero value is a detached node ready to be
// inserted into a list.
type Node[T any] struct {
	Value T

	next, prev *Node[T]
	list       *List[T]
}

// Next returns the next node, or nil if n is the last one or detached.
func (n *Node[T]) Next() *Node[T] {
	if l := n.list; l != nil && n.next != &l.root {
		return n.next
	}
	return nil
}

// Prev returns the previous node, or nil if n is the first one or detached.
func (n *Node[T]) Prev() *Node[T] {
	if l := n.list; l != nil && n.prev != &l.root {
		return n.prev
	}
	return nil
}

// Detach removes n from the list it belongs to. It is a no-op if n is
// already detached.
func (n *Node[T]) Detach() {
	if n.list == nil {
		return
	}

	n.prev.next = n.next
	n.next.prev = n.prev
	n.next = nil
	n.prev = nil
	n.list = nil
}

// List is a doubly linked list. Lists must be created with [New].
type List[T any] struct {
	root Node[T] // sentinel closing the ring; its Value and list fields are unused
}

// New returns an initialized empty list.
func New[T any]() *List[T] {
	l := &List[T]{}
	l.root.next = &l.root
	l.root.prev = &l.root
	return l
}

// Front returns the first node, or nil if the list is empty.
func (l *List[T]) Front() *Node[T] {
	if n := l.root.next; n != &l.root {
		return n
	}
	return nil
}

// Back returns the last node, or nil if the list is empty.
func (l *List[T]) Back() *Node[T] {
	if n := l.root.prev; n != &l.root {
		return n
	}
	return nil
}

// PushBack inserts n at the back of the list. n can be a detached node or a
// node of l, in which case it is moved to the new position. Pushing a node of
// another list is a no-op - detach it first.
func (l *List[T]) PushBack(n *Node[T]) {
	l.insertAfter(n, l.root.prev)
}

// PushFront inserts n at the front of the list. Same contract for n as
// [List.PushBack].
func (l *List[T]) PushFront(n *Node[T]) {
	l.insertAfter(n, &l.root)
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

// mark can be the sentinel
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

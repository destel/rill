package rill

import (
	"github.com/destel/rill/internal/core"
	"github.com/destel/rill/internal/heapbuffer"
)

func PriorityQueue[A any](in <-chan Try[A], capacity int, less func(A, A) bool) <-chan Try[A] {
	buf := heapbuffer.New(capacity, func(item1, item2 Try[A]) bool {
		// Always prioritize errors
		if item1.Error != nil {
			return true
		}
		if item2.Error != nil {
			return false
		}

		// invert the comparison to get max-heap behavior
		return !less(item1.Value, item2.Value)
	})

	return core.CustomBuffer[Try[A]](in, buf)
}

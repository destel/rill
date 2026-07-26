package core

import (
	"testing"
	"testing/synctest"

	"github.com/destel/rill/internal/th"
)

func TestDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := th.FromRange(0, 100)
		Drain(in)
		th.ExpectDrainedChan(t, in)
	})
}

func TestDiscard(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := make(chan int)
		Discard(in) // doesn't block

		// able to write
		in <- 1
		in <- 2
		close(in)
	})
}

func TestBuffer(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, Buffer[string](nil, 2), nil)
	})

	th.RunSynctest(t, "capacity", func(t *testing.T) {
		trySend := func(ch chan<- int, x int) bool {
			// Wait for the bubble to settle. It makes the non-blocking send deterministic:
			// it succeeds iff the buffer has room.
			synctest.Wait()
			select {
			case ch <- x:
				return true
			default:
				return false
			}
		}

		in := make(chan int)
		out := Buffer(in, 10)

		// try to write as much as possible w/o any consumer attached
		for i := range 1000 {
			if !trySend(in, i) {
				break
			}
		}
		close(in)

		// consume all
		outSlice := th.ToSlice(out)

		// Expecting 11 items, since one more item was buffered on the goroutine stack
		th.ExpectSlice(t, outSlice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	})

	// Zero is a valid capacity: the result is an unbuffered passthrough.
	th.RunSynctest(t, "zero size", func(t *testing.T) {
		in := th.FromRange(0, 10)
		out := Buffer(in, 0)
		th.ExpectSlice(t, th.ToSlice(out), []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	})
}

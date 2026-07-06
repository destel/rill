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
	synctest.Test(t, func(t *testing.T) {
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
		inBuf := Buffer(in, 2)

		th.ExpectValue(t, trySend(in, 1), true)
		th.ExpectValue(t, trySend(in, 2), true)
		th.ExpectValue(t, trySend(in, 3), false)

		x, ok := <-inBuf
		th.ExpectValue(t, x, 1)
		th.ExpectValue(t, ok, true)

		th.ExpectValue(t, trySend(in, 4), true)

		close(in)
		inSlice := th.ToSlice(inBuf)
		th.ExpectSlice(t, inSlice, []int{2, 4})
	})
}

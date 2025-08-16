package core

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestDrain(t *testing.T) {
	in := th.FromRange(0, 100)
	Drain(in)
	th.ExpectDrainedChan(t, in)
}

func TestDiscard(t *testing.T) {
	th.ExpectNotHang(t, 10*time.Second, func() {
		in := make(chan int)
		Discard(in)

		// able write in the main goroutine
		in <- 1
		in <- 2
		close(in)
	})
}

func TestBuffer(t *testing.T) {
	trySend := func(ch chan<- int, x int) bool {
		select {
		case ch <- x:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}

	in := make(chan int)
	inBuf := Buffer(in, 2)
	_ = inBuf

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
}

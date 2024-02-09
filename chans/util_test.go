package chans

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestDrainNB(t *testing.T) {
	t.Run("close before drain", func(t *testing.T) {
		th.ExpectNotHang(t, 10*time.Second, func() {
			in := make(chan int, 2)
			in <- 1
			in <- 2
			close(in)

			DrainNB(in)

			th.ExpectClosedChan(t, in, 1*time.Second)
		})
	})

	t.Run("close after drain", func(t *testing.T) {
		th.ExpectNotHang(t, 10*time.Second, func() {
			in := make(chan int, 2)
			in <- 1
			in <- 2

			DrainNB(in)

			in <- 3
			in <- 4
			in <- 5
			close(in)

			th.ExpectClosedChan(t, in, 1*time.Second)
		})
	})
}

func TestBuffer(t *testing.T) {
	trySend := func(ch chan<- int, x int) bool {
		select {
		case ch <- x:
			return true
		case <-time.After(1 * time.Second):
			return false
		}
	}

	in := make(chan int)
	inBuf := Buffer(in, 2)
	_ = inBuf

	th.ExpectValue(t, trySend(in, 1), true)
	th.ExpectValue(t, trySend(in, 2), true)
	th.ExpectValue(t, trySend(in, 3), false)
	th.ExpectValue(t, trySend(in, 4), false)
}

func TestFromToSlice(t *testing.T) {
	t.Run("correctness", func(t *testing.T) {
		inSlice := make([]int, 20)
		for i := 0; i < 20; i++ {
			inSlice[i] = i
		}

		outSlice := ToSlice(FromSlice(inSlice))

		th.ExpectSlice(t, outSlice, inSlice)
	})

	t.Run("ordering", func(t *testing.T) {
		inSlice := make([]int, 20000)
		for i := 0; i < 20000; i++ {
			inSlice[i] = i
		}

		outSlice := ToSlice(FromSlice(inSlice))

		th.ExpectSorted(t, outSlice)
	})

}

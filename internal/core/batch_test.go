package core

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestBatch(t *testing.T) {
	addDelayAfter := func(in <-chan int, value int, delay time.Duration) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for x := range in {
				out <- x
				if x == value {
					time.Sleep(delay)
				}
			}
		}()
		return out
	}

	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, Batch[string](nil, 10, 10*time.Second), nil)
	})

	th.RunSynctest(t, "empty", func(t *testing.T) {
		in := make(chan int)
		close(in)

		out := Batch(in, 10, 5*time.Second)

		outSlice := th.ToSlice(out)
		th.ExpectValue(t, len(outSlice), 0)
	})

	th.RunSynctest(t, "slow input", func(t *testing.T) {
		in := th.FromRange(0, 10)
		in = addDelayAfter(in, 2, 1*time.Second)
		in = addDelayAfter(in, 5, 10*time.Second)
		in = addDelayAfter(in, 8, 10*time.Second)

		out := Batch(in, 4, 5*time.Second)

		outSlice := th.ToSlice(out)

		th.ExpectValue(t, len(outSlice), 4)
		th.ExpectSlice(t, outSlice[0], []int{0, 1, 2, 3})
		th.ExpectSlice(t, outSlice[1], []int{4, 5})
		th.ExpectSlice(t, outSlice[2], []int{6, 7, 8})
		th.ExpectSlice(t, outSlice[3], []int{9})
	})

	th.RunSynctest(t, "fast input", func(t *testing.T) {
		in := th.FromRange(0, 10)
		in = addDelayAfter(in, 2, 1*time.Second)
		in = addDelayAfter(in, 5, 1*time.Second)
		in = addDelayAfter(in, 8, 1*time.Second)

		out := Batch(in, 4, 5*time.Second)
		outSlice := th.ToSlice(out)
		th.ExpectValue(t, len(outSlice), 3)
		th.ExpectSlice(t, outSlice[0], []int{0, 1, 2, 3})
		th.ExpectSlice(t, outSlice[1], []int{4, 5, 6, 7})
		th.ExpectSlice(t, outSlice[2], []int{8, 9})
	})

	th.RunSynctest(t, "no timeout", func(t *testing.T) {
		in := th.FromRange(0, 10)
		in = addDelayAfter(in, 1, 30*time.Second)
		in = addDelayAfter(in, 5, 30*time.Second)

		out := Batch(in, 4, -1)
		outSlice := th.ToSlice(out)
		th.ExpectValue(t, len(outSlice), 3)
		th.ExpectSlice(t, outSlice[0], []int{0, 1, 2, 3})
		th.ExpectSlice(t, outSlice[1], []int{4, 5, 6, 7})
		th.ExpectSlice(t, outSlice[2], []int{8, 9})
	})
}

func TestUnbatch(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, Unbatch[string](nil), nil)
	})

	th.RunSynctest(t, "normal", func(t *testing.T) {
		in := th.FromSlice([][]int{{1, 2, 3}, {4}, {5, 6, 7, 8}, {9, 10}})

		out := Unbatch(in)
		outSlice := th.ToSlice(out)

		th.ExpectSlice(t, outSlice, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	})
}

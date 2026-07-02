package core

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestBatch(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, Batch[string](nil, 10, 10*time.Second), nil)
	})

	th.RunSynctest(t, th.Name("empty"), func(t *testing.T) {
		in := make(chan int)
		go func() {
			time.Sleep(10 * time.Second)
			defer close(in)
		}()

		out := Batch(in, 10, 5*time.Second)

		outSlice := th.ToSlice(out)
		th.ExpectValue(t, len(outSlice), 0)
	})

	th.RunSynctest(t, th.Name("timeout"), func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)

			time.Sleep(1 * time.Second)
			th.Send(in, 1, 2)
			time.Sleep(1 * time.Second)
			th.Send(in, 3, 4)

			time.Sleep(10 * time.Second)
			th.Send(in, 5, 6)

			time.Sleep(10 * time.Second)
			th.Send(in, 7, 8, 9)

			time.Sleep(10 * time.Second)
		}()

		out := Batch(in, 4, 5*time.Second)

		outSlice := th.ToSlice(out)
		th.ExpectValue(t, len(outSlice), 3)
		th.ExpectSlice(t, outSlice[0], []int{1, 2, 3, 4})
		th.ExpectSlice(t, outSlice[1], []int{5, 6})
		th.ExpectSlice(t, outSlice[2], []int{7, 8, 9})
	})

	th.RunSynctest(t, th.Name("timeout no sleeps"), func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			th.Send(in, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, 5*time.Second)
		outSlice := th.ToSlice(out)
		th.ExpectValue(t, len(outSlice), 3)
		th.ExpectSlice(t, outSlice[0], []int{1, 2, 3, 4})
		th.ExpectSlice(t, outSlice[1], []int{5, 6, 7, 8})
		th.ExpectSlice(t, outSlice[2], []int{9, 10})
	})

	th.RunSynctest(t, th.Name("no timeout"), func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			th.Send(in, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, -1)
		outSlice := th.ToSlice(out)
		th.ExpectValue(t, len(outSlice), 3)
		th.ExpectSlice(t, outSlice[0], []int{1, 2, 3, 4})
		th.ExpectSlice(t, outSlice[1], []int{5, 6, 7, 8})
		th.ExpectSlice(t, outSlice[2], []int{9, 10})
	})
}

func TestUnbatch(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, Unbatch[string](nil), nil)
	})

	th.RunSynctest(t, th.Name("normal"), func(t *testing.T) {
		in := make(chan []int)
		go func() {
			defer close(in)
			th.Send(in, []int{1, 2, 3}, []int{4}, []int{5, 6, 7, 8}, []int{9, 10})
		}()

		out := Unbatch(in)
		outSlice := th.ToSlice(out)

		th.ExpectSlice(t, outSlice, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	})
}

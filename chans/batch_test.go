package chans

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestBatch(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var nilChan chan []string
		th.ExpectValue(t, Batch(nilChan, 10, 10*time.Second), nil)
	})

	t.Run("fast", func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			th.Send(in, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, 500*time.Millisecond)

		outSlice := ToSlice(out)
		th.ExpectValue(t, len(outSlice), 3)
		th.ExpectSlice(t, outSlice[0], []int{1, 2, 3, 4})
		th.ExpectSlice(t, outSlice[1], []int{5, 6, 7, 8})
		th.ExpectSlice(t, outSlice[2], []int{9, 10})
	})

	t.Run("slow", func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			th.Send(in, 1, 2, 3, 4, 5)
			time.Sleep(1 * time.Second)
			th.Send(in, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, 500*time.Millisecond)

		outSlice := ToSlice(out)
		th.ExpectValue(t, len(outSlice), 4)
		th.ExpectSlice(t, outSlice[0], []int{1, 2, 3, 4})
		th.ExpectSlice(t, outSlice[1], []int{5})
		th.ExpectSlice(t, outSlice[2], []int{6, 7, 8, 9})
		th.ExpectSlice(t, outSlice[3], []int{10})
	})

	t.Run("slow wo timeout", func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			th.Send(in, 1, 2, 3, 4, 5)
			time.Sleep(1 * time.Second)
			th.Send(in, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, -1)

		outSlice := ToSlice(out)
		th.ExpectValue(t, len(outSlice), 3)
		th.ExpectSlice(t, outSlice[0], []int{1, 2, 3, 4})
		th.ExpectSlice(t, outSlice[1], []int{5, 6, 7, 8})
		th.ExpectSlice(t, outSlice[2], []int{9, 10})
	})

	for _, timeout := range []time.Duration{-1, 10 * time.Second} {
		t.Run(th.Name("ordering", timeout), func(t *testing.T) {
			in := th.FromRange(0, 20000)

			out := Batch(in, 1000, timeout)

			ForEach(out, 1, func(batch []int) bool {
				th.ExpectSorted(t, batch)
				return !t.Failed()
			})
		})
	}
}

func TestUnbatch(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var nilChan chan []string
		th.ExpectValue(t, Unbatch(nilChan), nil)
	})

	t.Run("normal", func(t *testing.T) {
		in := make(chan []int)
		go func() {
			defer close(in)
			th.Send(in, []int{1, 2, 3}, []int{4}, []int{5, 6, 7, 8}, []int{9, 10})
		}()

		out := Unbatch(in)
		outSlice := ToSlice(out)

		th.ExpectSlice(t, outSlice, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	})
}

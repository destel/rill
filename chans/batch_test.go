package chans

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestBatch(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var nilChan chan []string
		th.ExpectValue(t, nil, Batch(nilChan, 10, 10*time.Second))
	})

	t.Run("fast", func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			th.Send(in, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, 500*time.Millisecond)

		outSlice := ToSlice(out)
		th.ExpectValue(t, 3, len(outSlice))
		th.ExpectSlice(t, []int{1, 2, 3, 4}, outSlice[0])
		th.ExpectSlice(t, []int{5, 6, 7, 8}, outSlice[1])
		th.ExpectSlice(t, []int{9, 10}, outSlice[2])
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
		th.ExpectValue(t, 4, len(outSlice))
		th.ExpectSlice(t, []int{1, 2, 3, 4}, outSlice[0])
		th.ExpectSlice(t, []int{5}, outSlice[1])
		th.ExpectSlice(t, []int{6, 7, 8, 9}, outSlice[2])
		th.ExpectSlice(t, []int{10}, outSlice[3])
	})

	t.Run("slow w/o timeout", func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			th.Send(in, 1, 2, 3, 4, 5)
			time.Sleep(1 * time.Second)
			th.Send(in, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, 0)

		outSlice := ToSlice(out)
		th.ExpectValue(t, 3, len(outSlice))
		th.ExpectSlice(t, []int{1, 2, 3, 4}, outSlice[0])
		th.ExpectSlice(t, []int{5, 6, 7, 8}, outSlice[1])
		th.ExpectSlice(t, []int{9, 10}, outSlice[2])
	})
}

func TestUnbatch(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var nilChan chan []string
		th.ExpectValue(t, nil, Unbatch(nilChan))
	})

	t.Run("normal", func(t *testing.T) {
		in := make(chan []int)
		go func() {
			defer close(in)
			th.Send(in, []int{1, 2, 3}, []int{4}, []int{5, 6, 7, 8}, []int{9, 10})
		}()

		out := Unbatch(in)
		outSlice := ToSlice(out)

		th.ExpectSlice(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, outSlice)
	})
}

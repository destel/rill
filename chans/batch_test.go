package chans

import (
	"testing"
	"time"
)

func TestBatch(t *testing.T) {
	var nilChan chan []string
	expectValue(t, nil, Batch(nilChan, 10, 10*time.Second))

	t.Run("fast", func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			send(in, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, 500*time.Millisecond)

		outSlice := ToSlice(out)
		expectValue(t, 3, len(outSlice))
		expectSlice(t, []int{1, 2, 3, 4}, outSlice[0])
		expectSlice(t, []int{5, 6, 7, 8}, outSlice[1])
		expectSlice(t, []int{9, 10}, outSlice[2])
	})

	t.Run("slow", func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			send(in, 1, 2, 3, 4, 5)
			time.Sleep(1 * time.Second)
			send(in, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, 500*time.Millisecond)

		outSlice := ToSlice(out)
		expectValue(t, 4, len(outSlice))
		expectSlice(t, []int{1, 2, 3, 4}, outSlice[0])
		expectSlice(t, []int{5}, outSlice[1])
		expectSlice(t, []int{6, 7, 8, 9}, outSlice[2])
		expectSlice(t, []int{10}, outSlice[3])
	})

	t.Run("slow w/o timeout", func(t *testing.T) {
		in := make(chan int)
		go func() {
			defer close(in)
			send(in, 1, 2, 3, 4, 5)
			time.Sleep(1 * time.Second)
			send(in, 6, 7, 8, 9, 10)
		}()

		out := Batch(in, 4, 0)

		outSlice := ToSlice(out)
		expectValue(t, 3, len(outSlice))
		expectSlice(t, []int{1, 2, 3, 4}, outSlice[0])
		expectSlice(t, []int{5, 6, 7, 8}, outSlice[1])
		expectSlice(t, []int{9, 10}, outSlice[2])
	})
}

func TestUnbatch(t *testing.T) {
	var nilChan chan []string
	expectValue(t, nil, Unbatch(nilChan))

	in := make(chan []int)
	go func() {
		defer close(in)
		in <- []int{1, 2, 3}
		in <- []int{4}
		in <- []int{5, 6, 7, 8}
		in <- []int{9, 10}
	}()

	out := Unbatch(in)
	outSlice := ToSlice(out)

	expectSlice(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, outSlice)
}

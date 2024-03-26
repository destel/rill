package core

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestInfiniteBuffer(t *testing.T) {
	in := make(chan int)
	out := infiniteBuffer(in)

	for i := 0; i < 1000; i++ {
		in <- i
	}
	close(in)

	i := -1
	for v := range out {
		i++
		th.ExpectValue(t, v, i)
	}
	th.ExpectValue(t, i, 1000-1)
}

func TestDelay(t *testing.T) {
	type Item struct {
		Value  int
		SentAt time.Time
	}

	t.Run("correctness", func(t *testing.T) {
		const delay = 5 * time.Second
		const eps = 1000 * time.Millisecond // Race detector slows down the execution. Need to use larger epsilon

		in := make(chan Item)
		out := Delay(in, delay)

		go func() {
			defer close(in)
			for i := 0; i < 100000; i++ {
				in <- Item{Value: i, SentAt: time.Now()}
			}
		}()

		i := -1
		for item := range out {
			i++
			th.ExpectValue(t, item.Value, i)
			th.ExpectValueInDelta(t, time.Since(item.SentAt), delay, eps)
			if t.Failed() {
				t.FailNow()
			}
		}
		th.ExpectValue(t, i, 100000-1)
	})

	t.Run("slow producer", func(t *testing.T) {
		const delay = 5 * time.Second
		const eps = 1000 * time.Millisecond

		in := make(chan Item)
		out := Delay(in, delay)

		go func() {
			defer close(in)
			for i := 0; i < 100; i++ {
				in <- Item{Value: i, SentAt: time.Now()}

				if i == 30 || i == 50 {
					time.Sleep(1 * time.Second) // make producer slow
				}
			}

			// let buffer be fully consumed, by the time we close the channel
			time.Sleep(3 * time.Second)
		}()

		i := -1
		for item := range out {
			i++
			th.ExpectValue(t, item.Value, i)
			th.ExpectValueInDelta(t, time.Since(item.SentAt), delay, eps)
		}
		th.ExpectValue(t, i, 100-1)
	})

	t.Run("slow consumer", func(t *testing.T) {
		const delay = 5 * time.Second

		in := make(chan Item)
		out := Delay(in, delay)

		go func() {
			defer close(in)
			for i := 0; i < 100; i++ {
				in <- Item{Value: i, SentAt: time.Now()}
			}
		}()

		i := -1
		for item := range out {
			i++
			th.ExpectValue(t, item.Value, i)

			if i == 30 || i == 50 {
				time.Sleep(1 * time.Second) // make consumer slow
			}

			// less strict condition than for the fast consumer
			th.ExpectValueGTE(t, time.Since(item.SentAt), delay)
		}
		th.ExpectValue(t, i, 100-1)
	})
}

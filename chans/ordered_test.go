package chans

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestOrderedMap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, OrderedMap(nil, 10, func(x int) int { return x }), nil)
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 20)
		out := OrderedMap(in, 5, func(x int) string {
			// break the ordering, make 8th element slow
			if x == 8 {
				time.Sleep(1 * time.Second)
			}

			return fmt.Sprintf("%02d", x)
		})

		outSlice := ToSlice(out)
		th.ExpectSlice(t, outSlice, []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19"})
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress th.InProgressCounter

		in := th.FromRange(0, 20)
		out := OrderedMap(in, 10, func(x int) int {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return x + 1
		})

		Drain(out)
		th.ExpectValue(t, inProgress.Max(), 10)
	})
}

func TestOrderedFilter(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, OrderedFilter(nil, 10, func(x int) bool { return true }), nil)
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 20)
		out := OrderedFilter(in, 5, func(x int) bool {
			// break the ordering, make 8th element slow
			if x == 8 {
				time.Sleep(1 * time.Second)
			}

			return x%2 == 0
		})

		outSlice := ToSlice(out)

		var expected []int
		for i := 0; i < 20; i += 2 {
			expected = append(expected, i)
		}

		th.ExpectSlice(t, outSlice, []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18})
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress th.InProgressCounter

		in := th.FromRange(0, 20)
		out := OrderedFilter(in, 10, func(x int) bool {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return x%2 == 0
		})

		Drain(out)
		th.ExpectValue(t, inProgress.Max(), 10)
	})
}

func TestOrderedFlatMap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, OrderedFlatMap(nil, 10, func(x int) <-chan string { return nil }), nil)
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 10)
		out := OrderedFlatMap(in, 5, func(x int) <-chan string {
			// break the ordering, make 8th element slow
			if x == 8 {
				time.Sleep(1 * time.Second)
			}

			return FromSlice([]string{
				fmt.Sprintf("%02dA", x),
				fmt.Sprintf("%02dB", x),
			})
		})

		outSlice := ToSlice(out)

		th.ExpectSlice(t, outSlice, []string{"00A", "00B", "01A", "01B", "02A", "02B", "03A", "03B", "04A", "04B", "05A", "05B", "06A", "06B", "07A", "07B", "08A", "08B", "09A", "09B"})
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress th.InProgressCounter

		in := th.FromRange(0, 20)
		out := OrderedFlatMap(in, 10, func(x int) <-chan int {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return th.FromRange(10*x, 10*(x+1))
		})

		Drain(out)
		th.ExpectValue(t, inProgress.Max(), 10)
	})
}

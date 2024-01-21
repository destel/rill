package chans

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestOrderedMap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, nil, OrderedMap(nil, 10, func(x int) int { return x }))
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 20)
		out := OrderedMap(in, 5, func(x int) string {
			// break the ordering
			if x == 8 {
				time.Sleep(1 * time.Second)
			}

			return fmt.Sprintf("%03d", x)
		})

		outSlice := ToSlice(out)

		var expected []string
		for i := 0; i < 20; i++ {
			expected = append(expected, fmt.Sprintf("%03d", i))
		}

		th.ExpectSlice(t, expected, outSlice)
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
		th.ExpectValue(t, 10, inProgress.Max())
	})
}

func TestOrderedFilter(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, nil, OrderedFilter(nil, 10, func(x int) bool { return true }))
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 20)
		out := OrderedFilter(in, 5, func(x int) bool {
			// break the ordering
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

		th.ExpectSlice(t, expected, outSlice)
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
		th.ExpectValue(t, 10, inProgress.Max())
	})
}

func TestOrderedFlatMap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, nil, OrderedFlatMap(nil, 10, func(x int) <-chan string { return nil }))
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 20)
		out := OrderedFlatMap(in, 5, func(x int) <-chan string {
			// break the ordering
			if x == 8 {
				time.Sleep(1 * time.Second)
			}

			return FromSlice([]string{
				fmt.Sprintf("%03dA", x),
				fmt.Sprintf("%03dB", x),
				fmt.Sprintf("%03dC", x),
			})
		})

		outSlice := ToSlice(out)

		var expected []string
		for i := 0; i < 20; i++ {
			expected = append(expected, fmt.Sprintf("%03dA", i))
			expected = append(expected, fmt.Sprintf("%03dB", i))
			expected = append(expected, fmt.Sprintf("%03dC", i))
		}

		th.ExpectSlice(t, expected, outSlice)
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
		th.ExpectValue(t, 10, inProgress.Max())
	})
}
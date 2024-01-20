package chans

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

func TestMap(t *testing.T) {
	t.Run("correctness", func(t *testing.T) {
		in := fromRange(0, 20)

		out := Map(in, 5, func(x int) string {
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

		if sort.StringsAreSorted(outSlice) {
			t.Errorf("expected outSlice to be unsorted")
		}

		sort.Strings(outSlice)
		expectSlice(t, expected, outSlice)
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress inProgressCounter

		in := fromRange(0, 20)

		out := Map(in, 10, func(x int) int {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return x + 1
		})

		_ = ToSlice(out)

		expectValue(t, 10, inProgress.Max())
	})
}

func TestFilter(t *testing.T) {
	t.Run("correctness", func(t *testing.T) {
		in := fromRange(0, 20)

		out := Filter(in, 5, func(x int) bool {
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

		if sort.IntsAreSorted(outSlice) {
			t.Errorf("expected outSlice to be unsorted")
		}

		sort.Ints(outSlice)
		expectSlice(t, expected, outSlice)
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress inProgressCounter

		in := fromRange(0, 20)

		out := Filter(in, 10, func(x int) bool {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return x%2 == 0
		})

		_ = ToSlice(out)

		expectValue(t, 10, inProgress.Max())
	})
}

func TestFlatMap(t *testing.T) {
	t.Run("correctness", func(t *testing.T) {
		in := fromRange(0, 20)

		out := FlatMap(in, 5, func(x int) <-chan string {
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

		if sort.StringsAreSorted(outSlice) {
			t.Errorf("expected outSlice to be unsorted")
		}

		sort.Strings(outSlice)
		expectSlice(t, expected, outSlice)
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress inProgressCounter

		in := fromRange(0, 20)

		out := FlatMap(in, 10, func(x int) <-chan int {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return fromRange(10*x, 10*(x+1))
		})

		_ = ToSlice(out)

		expectValue(t, 10, inProgress.Max())
	})
}

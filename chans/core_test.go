package chans

import (
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestMap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, Map(nil, 10, func(x int) int { return x }), nil)
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 20)
		out := Map(in, 5, func(x int) string {
			// break the ordering, make 8th element slow
			if x == 8 {
				time.Sleep(1 * time.Second)
			}

			return fmt.Sprintf("%02d", x)
		})

		outSlice := ToSlice(out)

		if sort.StringsAreSorted(outSlice) {
			t.Errorf("expected outSlice to be unsorted")
		}

		sort.Strings(outSlice)
		th.ExpectSlice(t, outSlice, []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19"})
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress th.InProgressCounter

		in := th.FromRange(0, 20)
		out := Map(in, 10, func(x int) int {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return x + 1
		})

		Drain(out)
		th.ExpectValue(t, inProgress.Max(), 10)
	})
}

func TestFilter(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, Filter(nil, 10, func(x int) bool { return true }), nil)
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 20)
		out := Filter(in, 5, func(x int) bool {
			// break the ordering, make 8th element slow
			if x == 8 {
				time.Sleep(1 * time.Second)
			}

			return x%2 == 0
		})

		outSlice := ToSlice(out)

		if sort.IntsAreSorted(outSlice) {
			t.Errorf("expected outSlice to be unsorted")
		}

		sort.Ints(outSlice)
		th.ExpectSlice(t, outSlice, []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18})
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress th.InProgressCounter

		in := th.FromRange(0, 20)
		out := Filter(in, 10, func(x int) bool {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return x%2 == 0
		})

		Drain(out)
		th.ExpectValue(t, inProgress.Max(), 10)
	})
}

func TestFlatMap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		th.ExpectValue(t, FlatMap(nil, 10, func(x int) <-chan string { return nil }), nil)
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 10)
		out := FlatMap(in, 5, func(x int) <-chan string {
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

		if sort.StringsAreSorted(outSlice) {
			t.Errorf("expected outSlice to be unsorted")
		}

		sort.Strings(outSlice)
		th.ExpectSlice(t, outSlice, []string{"00A", "00B", "01A", "01B", "02A", "02B", "03A", "03B", "04A", "04B", "05A", "05B", "06A", "06B", "07A", "07B", "08A", "08B", "09A", "09B"})
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress th.InProgressCounter

		in := th.FromRange(0, 20)
		out := FlatMap(in, 10, func(x int) <-chan int {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return th.FromRange(0, 5)
		})

		Drain(out)
		th.ExpectValue(t, inProgress.Max(), 10)
	})
}

func TestForEach(t *testing.T) {
	testCorrectness := func(t *testing.T, n int) {
		t.Run(fmt.Sprintf("correctness n=%d", n), func(t *testing.T) {
			sum := int64(0)

			in := th.FromRange(0, 20)
			ForEach(in, n, func(x int) bool {
				atomic.AddInt64(&sum, int64(x))
				return true
			})

			th.ExpectValue(t, sum, int64(19*20/2))
		})
	}

	testEarlyExit := func(t *testing.T, n int) {
		t.Run(fmt.Sprintf("early exit n=%d", n), func(t *testing.T) {
			th.NotHang(t, 10*time.Second, func() {
				done := make(chan struct{})
				defer close(done)

				in := th.InfiniteChan(done)

				defer DrainNB(in)

				ForEach(in, n, func(x int) bool {
					if x == 100 {
						return false
					}
					return true
				})
			})
		})
	}

	testConcurrency := func(t *testing.T, n int) {
		t.Run(fmt.Sprintf("concurrency n=%d", n), func(t *testing.T) {
			var inProgress th.InProgressCounter

			in := th.FromRange(0, 2*n)
			ForEach(in, n, func(x int) bool {
				inProgress.Inc()
				defer inProgress.Dec()

				time.Sleep(1 * time.Second)
				return true
			})

			th.ExpectValue(t, inProgress.Max(), n)
		})
	}

	testCorrectness(t, 1)
	testEarlyExit(t, 1)
	testConcurrency(t, 1)

	testCorrectness(t, 5)
	testEarlyExit(t, 5)
	testConcurrency(t, 5)
}

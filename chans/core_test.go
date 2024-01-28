package chans

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func testname(name string, ordered bool, n int) string {
	if ordered {
		return fmt.Sprintf("%s_ordered_%d", name, n)
	} else {
		return fmt.Sprintf("%s_%d", name, n)
	}
}

func doMap[A, B any](ord bool, in <-chan A, n int, f func(A) B) <-chan B {
	if ord {
		return OrderedMap(in, n, f)
	}
	return Map(in, n, f)
}

func TestMap(t *testing.T) {
	for _, ord := range []bool{false, true} {
		for _, n := range []int{1, 5} {

			t.Run(testname("nil", ord, n), func(t *testing.T) {
				out := doMap(ord, nil, n, func(x int) int { return x })
				th.ExpectValue(t, out, nil)
			})

			t.Run(testname("correctness", ord, n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := doMap(ord, in, n, func(x int) string {
					// break the ordering, make 8th element slow
					if x == 8 {
						time.Sleep(1 * time.Second)
					}

					return fmt.Sprintf("%03d", x)
				})

				outSlice := ToSlice(out)
				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}

				expected := make([]string, 0, 20)
				for i := 0; i < 20; i++ {
					expected = append(expected, fmt.Sprintf("%03d", i))
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expected)
			})

			t.Run(testname("concurrency", ord, n), func(t *testing.T) {
				var inProgress th.InProgressCounter

				in := th.FromRange(0, n*2)
				out := doMap(ord, in, n, func(x int) int {
					inProgress.Inc()
					defer inProgress.Dec()

					time.Sleep(1 * time.Second)
					return x + 1
				})

				Drain(out)
				th.ExpectValue(t, inProgress.Max(), n)
			})

		}
	}
}

func doFilter[A any](ord bool, in <-chan A, n int, f func(A) bool) <-chan A {
	if ord {
		return OrderedFilter(in, n, f)
	}
	return Filter(in, n, f)
}

func TestFilter(t *testing.T) {
	for _, ord := range []bool{false, true} {
		for _, n := range []int{1, 5} {

			t.Run(testname("nil", ord, n), func(t *testing.T) {
				out := doFilter(ord, nil, n, func(x int) bool { return true })
				th.ExpectValue(t, out, nil)
			})

			t.Run(testname("correctness", ord, n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := doFilter(ord, in, n, func(x int) bool {
					// break the ordering, make 8th element slow
					if x == 8 {
						time.Sleep(1 * time.Second)
					}

					return x%2 == 0
				})

				outSlice := ToSlice(out)
				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}

				expected := make([]int, 0, 20)
				for i := 0; i < 20; i += 2 {
					expected = append(expected, i)
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expected)
			})

			t.Run(testname("concurrency", ord, n), func(t *testing.T) {
				var inProgress th.InProgressCounter

				in := th.FromRange(0, n*2)
				out := doFilter(ord, in, n, func(x int) bool {
					inProgress.Inc()
					defer inProgress.Dec()

					time.Sleep(1 * time.Second)
					return x%2 == 0
				})

				Drain(out)
				th.ExpectValue(t, inProgress.Max(), n)
			})

		}
	}
}

func doFlatMap[A, B any](ord bool, in <-chan A, n int, f func(A) <-chan B) <-chan B {
	if ord {
		return OrderedFlatMap(in, n, f)
	}
	return FlatMap(in, n, f)
}

func TestFlatMap(t *testing.T) {
	for _, ord := range []bool{false, true} {
		for _, n := range []int{1, 5} {

			t.Run(testname("nil", ord, n), func(t *testing.T) {
				out := doFlatMap(ord, nil, n, func(x int) <-chan string { return nil })
				th.ExpectValue(t, out, nil)
			})

			t.Run(testname("correctness", ord, n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := doFlatMap(ord, in, n, func(x int) <-chan string {
					// break the ordering, make 8th element slow
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
				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}

				expected := make([]string, 0, 20)
				for i := 0; i < 20; i++ {
					expected = append(expected, fmt.Sprintf("%03dA", i), fmt.Sprintf("%03dB", i), fmt.Sprintf("%03dC", i))
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expected)
			})

			t.Run(testname("concurrency", ord, n), func(t *testing.T) {
				var inProgress th.InProgressCounter

				in := th.FromRange(0, 2*n)
				out := doFlatMap(ord, in, n, func(x int) <-chan int {
					inProgress.Inc()
					defer inProgress.Dec()

					time.Sleep(1 * time.Second)
					return th.FromRange(0, 5)
				})

				Drain(out)
				th.ExpectValue(t, inProgress.Max(), n)
			})

		}
	}
}

func TestForEach(t *testing.T) {
	for _, n := range []int{1, 5} {

		t.Run(testname("correctness", false, n), func(t *testing.T) {
			sum := int64(0)

			in := th.FromRange(0, 20)
			ForEach(in, n, func(x int) bool {
				atomic.AddInt64(&sum, int64(x))
				return true
			})

			th.ExpectValue(t, sum, int64(19*20/2))
		})

		t.Run(testname("early_exit", false, n), func(t *testing.T) {
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

		t.Run(testname("concurrency", false, n), func(t *testing.T) {
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
}

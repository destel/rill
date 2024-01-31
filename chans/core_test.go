package chans

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func testname(name string, ordered bool, n int) string {
	res := name
	if ordered {
		res = res + "_ordered"
	} else if name == "ordering" {
		res = res + "_unordered"
	}
	if n > 0 {
		res = res + "_" + fmt.Sprint(n)
	}
	return res
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
					return fmt.Sprintf("%03d", x)
				})

				outSlice := ToSlice(out)

				expectedSlice := make([]string, 0, 20)
				for i := 0; i < 20; i++ {
					expectedSlice = append(expectedSlice, fmt.Sprintf("%03d", i))
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expectedSlice)
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

		t.Run(testname("ordering", ord, 0), func(t *testing.T) {
			in := th.FromRange(0, 10000)

			out := doMap(ord, in, 50, func(x int) int {
				return x
			})

			outSlice := ToSlice(out)

			if ord {
				th.ExpectSorted(t, outSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
			}
		})

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
					return x%2 == 0
				})

				outSlice := ToSlice(out)

				expectedSlice := make([]int, 0, 20)
				for i := 0; i < 20; i += 2 {
					expectedSlice = append(expectedSlice, i)
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expectedSlice)
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

		t.Run(testname("ordering", ord, 0), func(t *testing.T) {
			in := th.FromRange(0, 10000)

			out := doFilter(ord, in, 50, func(x int) bool {
				return x%2 == 0
			})

			outSlice := ToSlice(out)

			if ord {
				th.ExpectSorted(t, outSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
			}
		})
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
					return FromSlice([]string{
						fmt.Sprintf("%03dA", x),
						fmt.Sprintf("%03dB", x),
						fmt.Sprintf("%03dC", x),
					})
				})

				outSlice := ToSlice(out)

				expectedSlice := make([]string, 0, 20)
				for i := 0; i < 20; i++ {
					expectedSlice = append(expectedSlice, fmt.Sprintf("%03dA", i), fmt.Sprintf("%03dB", i), fmt.Sprintf("%03dC", i))
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expectedSlice)
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

		t.Run(testname("ordering", ord, 0), func(t *testing.T) {
			in := th.FromRange(0, 10000)

			out := doFlatMap(ord, in, 50, func(x int) <-chan string {
				return FromSlice([]string{
					fmt.Sprintf("%06dA", x),
					fmt.Sprintf("%06dB", x),
					fmt.Sprintf("%06dC", x),
				})
			})

			outSlice := ToSlice(out)

			if ord {
				th.ExpectSorted(t, outSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
			}
		})
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
			th.ExpectNotHang(t, 10*time.Second, func() {
				done := make(chan struct{})
				defer close(done)

				in := th.InfiniteChan(done)

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

	t.Run("ordering_1", func(t *testing.T) {
		in := th.FromRange(0, 10000)

		prev := -1
		ForEach(in, 1, func(x int) bool {
			if x < prev {
				t.Errorf("expected ordered processing")
				return false
			}
			prev = x
			return true
		})
	})

}

// Compare ordered and unordered map in a single threaded scenario
func BenchmarkCompareMaps(b *testing.B) {
	b.Run("unordered", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			in := th.FromRange(0, 10000)
			out := Map(in, 1, func(x int) int {
				return x
			})
			Drain(out)
		}
	})

	b.Run("ordered", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			in := th.FromRange(0, 10000)
			out := OrderedMap(in, 1, func(x int) int {
				return x
			})
			Drain(out)
		}
	})
}

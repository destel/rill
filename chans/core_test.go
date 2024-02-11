package chans

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func universalMap[A, B any](ord bool, in <-chan A, n int, f func(A) B) <-chan B {
	if ord {
		return OrderedMap(in, n, f)
	}
	return Map(in, n, f)
}

func TestMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalMap(ord, nil, n, func(x int) int { return x })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := universalMap(ord, in, n, func(x int) string {
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

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				out := universalMap(ord, in, n, func(x int) int {
					return x
				})

				outSlice := ToSlice(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})
		}
	})
}

func universalFilter[A any](ord bool, in <-chan A, n int, f func(A) bool) <-chan A {
	if ord {
		return OrderedFilter(in, n, f)
	}
	return Filter(in, n, f)
}

func TestFilter(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalFilter(ord, nil, n, func(x int) bool { return true })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := universalFilter(ord, in, n, func(x int) bool {
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

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				out := universalFilter(ord, in, n, func(x int) bool {
					return x%2 == 0
				})

				outSlice := ToSlice(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		}

	})
}

func universalFlatMap[A, B any](ord bool, in <-chan A, n int, f func(A) <-chan B) <-chan B {
	if ord {
		return OrderedFlatMap(in, n, f)
	}
	return FlatMap(in, n, f)
}

func TestFlatMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalFlatMap(ord, nil, n, func(x int) <-chan string { return nil })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := universalFlatMap(ord, in, n, func(x int) <-chan string {
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

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				out := universalFlatMap(ord, in, n, func(x int) <-chan string {
					return FromSlice([]string{
						fmt.Sprintf("%06dA", x),
						fmt.Sprintf("%06dB", x),
						fmt.Sprintf("%06dC", x),
					})
				})

				outSlice := ToSlice(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		}
	})
}

func TestForEach(t *testing.T) {
	for _, n := range []int{1, 5} {

		t.Run(th.Name("correctness", n), func(t *testing.T) {
			sum := int64(0)

			in := th.FromRange(0, 20)
			ForEach(in, n, func(x int) bool {
				atomic.AddInt64(&sum, int64(x))
				return true
			})

			th.ExpectValue(t, sum, int64(19*20/2))
		})

		t.Run(th.Name("early exit", n), func(t *testing.T) {
			th.ExpectNotHang(t, 10*time.Second, func() {
				done := make(chan struct{})
				defer close(done)

				sum := int64(0)

				in := th.InfiniteChan(done)

				ForEach(in, n, func(x int) bool {
					if x == 100 {
						return false
					}
					atomic.AddInt64(&sum, int64(x))
					return true
				})

				if sum < 99*100/2 {
					t.Errorf("expected at least 100 iterations to complete")
				}
			})
		})

		t.Run(th.Name("ordering", n), func(t *testing.T) {
			in := th.FromRange(0, 20000)

			var mu sync.Mutex
			outSlice := make([]int, 0, 20000)

			ForEach(in, n, func(x int) bool {
				mu.Lock()
				outSlice = append(outSlice, x)
				mu.Unlock()
				return true
			})

			if n == 1 {
				th.ExpectSorted(t, outSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
			}
		})

	}

	t.Run("deterministic when n=1", func(t *testing.T) {
		in := th.FromRange(0, 100)

		maxX := -1

		ForEach(in, 1, func(x int) bool {
			if x == 10 {
				return false
			}

			if x > maxX {
				maxX = x
			}

			return true
		})

		th.ExpectValue(t, maxX, 9)
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

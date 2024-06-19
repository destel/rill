package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestReduce(t *testing.T) {
	for _, n := range []int{1, 4, 8} {
		t.Run(th.Name("nil", n), func(t *testing.T) {
			n := n
			th.ExpectHang(t, 1*time.Second, func() {
				_, _ = Reduce[int](nil, n, func(a, b int) int {
					return a + b
				})
			})
		})

		t.Run(th.Name("empty", n), func(t *testing.T) {
			in := th.FromSlice([]int{})
			_, ok := Reduce(in, n, func(a, b int) int {
				return a + b
			})

			th.ExpectValue(t, ok, false)
		})

		t.Run(th.Name("correctness", n), func(t *testing.T) {
			in := th.FromRange(0, 100)
			out, ok := Reduce(in, n, func(a, b int) int {
				return a + b
			})

			th.ExpectValue(t, out, 99*100/2)
			th.ExpectValue(t, ok, true)
		})

		t.Run(th.Name("concurrency", n), func(t *testing.T) {
			in := th.FromRange(0, 100)

			monitor := th.NewConcurrencyMonitor(1 * time.Second)

			_, _ = Reduce(in, n, func(a, b int) int {
				monitor.Inc()
				defer monitor.Dec()

				return a + b
			})

			th.ExpectValue(t, monitor.Max(), n)
		})
	}
}

func TestMapReduce(t *testing.T) {
	for _, nm := range []int{1, 4} {
		for _, nr := range []int{1, 4, 8} {
			t.Run(th.Name("nil", nm, nr), func(t *testing.T) {
				nm, nr := nm, nr
				th.ExpectHang(t, 1*time.Second, func() {
					var in chan int
					_ = MapReduce(in,
						nm, func(x int) (string, int) {
							return "", 1
						},
						nr, func(a, b int) int {
							return a + b
						},
					)
				})
			})

			t.Run(th.Name("empty", nm, nr), func(t *testing.T) {
				in := th.FromSlice([]int{})
				out := MapReduce(in,
					nm, func(x int) (string, int) {
						return fmt.Sprintf("%d mod 3", x%3), 1
					},
					nr, func(a, b int) int {
						return a + b
					},
				)

				th.ExpectMap(t, out, map[string]int{})
			})

			t.Run(th.Name("correctness", nm, nr), func(t *testing.T) {
				in := th.FromRange(0, 200)
				out := MapReduce(in,
					nm, func(x int) (string, int) {
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x
					},
					nr, func(a, b int) int {
						return a + b
					},
				)

				th.ExpectMap(t, out, map[string]int{
					"1-digit": (0 + 9) * 10 / 2,
					"2-digit": (10 + 99) * 90 / 2,
					"3-digit": (100 + 199) * 100 / 2,
				})
			})

			t.Run(th.Name("concurrency", nm, nr), func(t *testing.T) {
				// Need a really high number of items to reliably "catch" the max concurrency.
				in := th.FromRange(0, 100)

				mapMonitor := th.NewConcurrencyMonitor(1 * time.Second)
				reduceMonitor := th.NewConcurrencyMonitor(1 * time.Second)

				_ = MapReduce(in,
					nm, func(x int) (string, int) {
						mapMonitor.Inc()
						defer mapMonitor.Dec()

						return fmt.Sprintf("%d mod 3", x%3), 1
					},
					nr, func(a, b int) int {
						reduceMonitor.Inc()
						defer reduceMonitor.Dec()

						return a + b
					},
				)

				th.ExpectValue(t, mapMonitor.Max(), nm)
				th.ExpectValue(t, reduceMonitor.Max(), nr)

			})

		}
	}
}

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
			th.ExpectDeadlock(t, func() {
				_, _ = Reduce(nil, n, func(a, b int) int {
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

		th.RunSynctest(t, th.Name("concurrency", n), func(t *testing.T) {
			in := th.FromRange(0, 100)

			var monitor th.ConcurrencyMonitor

			_, _ = Reduce(in, n, func(a, b int) int {
				monitor.Enter()
				defer monitor.Exit()

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
				th.ExpectDeadlock(t, func() {
					_ = MapReduce(nil,
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

			th.RunSynctest(t, th.Name("concurrency", nm, nr), func(t *testing.T) {
				in := th.FromRange(0, 100)

				// To reach max concurrency in the reduce phase, the map phase must outpace it
				// rather than become the bottleneck. Under synctest we get that by giving
				// mappers a much smaller hold than reducers.
				mapMonitor := th.ConcurrencyMonitor{Hold: 1 * time.Second}
				reduceMonitor := th.ConcurrencyMonitor{Hold: 30 * time.Second}

				_ = MapReduce(in,
					nm, func(x int) (string, int) {
						mapMonitor.Enter()
						defer mapMonitor.Exit()

						return fmt.Sprintf("%d mod 3", x%3), 1
					},
					nr, func(a, b int) int {
						reduceMonitor.Enter()
						defer reduceMonitor.Exit()

						return a + b
					},
				)

				th.ExpectValue(t, mapMonitor.Max(), nm)
				th.ExpectValue(t, reduceMonitor.Max(), nr)
			})

		}
	}
}

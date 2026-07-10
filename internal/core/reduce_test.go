package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestReduce(t *testing.T) {
	th.TestLevels(t, []int{1, 4, 8}, func(t *testing.T, n int) {

		t.Run("nil", func(t *testing.T) {
			th.ExpectBlock(t, func(t *testing.T) {
				Reduce(nil, n, func(a, b int) int {
					return a + b
				})
			})
		})

		th.RunSynctest(t, "empty", func(t *testing.T) {
			in := th.FromSlice([]int{})
			_, ok := Reduce(in, n, func(a, b int) int {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return a + b
			})

			th.ExpectDrainedChan(t, in)

			th.ExpectValue(t, ok, false)
		})

		th.RunSynctest(t, "correctness", func(t *testing.T) {
			in := th.FromRange(0, 100)
			out, ok := Reduce(in, n, func(a, b int) int {
				th.SimulateWork(1*time.Second, 2*time.Second)
				return a + b
			})

			th.ExpectDrainedChan(t, in)

			th.ExpectValue(t, out, 99*100/2)
			th.ExpectValue(t, ok, true)
		})

		th.RunSynctest(t, "concurrency", func(t *testing.T) {
			in := th.FromRange(0, 100)

			var gauge th.InFlightGauge

			_, _ = Reduce(in, n, func(a, b int) int {
				gauge.Enter()
				defer gauge.Exit()
				th.SimulateWork(1*time.Second, 2*time.Second)

				return a + b
			})

			th.ExpectValue(t, gauge.Max(), n)
		})

	})
}

func TestMapReduce(t *testing.T) {
	th.TestVariants(t, "nm", []int{1, 4}, func(t *testing.T, nm int) {
		th.TestVariants(t, "nr", []int{1, 4, 8}, func(t *testing.T, nr int) {

			t.Run("nil", func(t *testing.T) {
				th.ExpectBlock(t, func(t *testing.T) {
					MapReduce(nil,
						nm, func(x int) (string, int) {
							return "", 1
						},
						nr, func(a, b int) int {
							return a + b
						},
					)
				})
			})

			th.RunSynctest(t, "empty", func(t *testing.T) {
				in := th.FromSlice([]int{})
				out := MapReduce(in,
					nm, func(x int) (string, int) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						return fmt.Sprintf("%d mod 3", x%3), 1
					},
					nr, func(a, b int) int {
						th.SimulateWork(10*time.Second, 20*time.Second)
						return a + b
					},
				)

				th.ExpectDrainedChan(t, in)

				th.ExpectMap(t, out, map[string]int{})
			})

			th.RunSynctest(t, "correctness", func(t *testing.T) {
				in := th.FromRange(0, 200)
				out := MapReduce(in,
					nm, func(x int) (string, int) {
						th.SimulateWork(1*time.Second, 2*time.Second)
						s := fmt.Sprint(x)
						return fmt.Sprintf("%d-digit", len(s)), x
					},
					nr, func(a, b int) int {
						th.SimulateWork(10*time.Second, 20*time.Second)
						return a + b
					},
				)

				th.ExpectDrainedChan(t, in)

				th.ExpectMap(t, out, map[string]int{
					"1-digit": (0 + 9) * 10 / 2,
					"2-digit": (10 + 99) * 90 / 2,
					"3-digit": (100 + 199) * 100 / 2,
				})
			})

			th.RunSynctest(t, "concurrency", func(t *testing.T) {
				in := th.FromRange(0, 100)

				// To reach max concurrency in the reduce phase, the map phase must outpace it
				// rather than become the bottleneck. Under synctest we get that by giving
				// mappers a much smaller work than reducers.
				var mapGauge th.InFlightGauge
				var reduceGauge th.InFlightGauge

				_ = MapReduce(in,
					nm, func(x int) (string, int) {
						mapGauge.Enter()
						defer mapGauge.Exit()
						th.SimulateWork(1*time.Second, 2*time.Second)

						return fmt.Sprintf("%d mod 3", x%3), 1
					},
					nr, func(a, b int) int {
						reduceGauge.Enter()
						defer reduceGauge.Exit()
						th.SimulateWork(10*time.Second, 20*time.Second)

						return a + b
					},
				)

				th.ExpectValue(t, mapGauge.Max(), nm)
				th.ExpectValue(t, reduceGauge.Max(), nr)
			})

		})
	})
}

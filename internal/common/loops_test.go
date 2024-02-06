package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func toSlice[A any](in <-chan A) []A {
	var out []A
	for x := range in {
		out = append(out, x)
	}
	return out
}

// calls Loop or OrderedLoop based on the value of ord
func universalLoop[A, B any](ord bool, in <-chan A, done chan<- B, n int, f func(a A, canWrite <-chan struct{})) {
	if ord {
		OrderedLoop(in, done, n, f)
	} else {
		canWrite := make(chan struct{}, n)
		close(canWrite)

		Loop(in, done, n, func(a A) {
			f(a, canWrite)
		})
	}
}

func TestLoops(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {

		for _, n := range []int{1, 5} {
			t.Run(fmt.Sprint("correctness_", n), func(t *testing.T) {
				in := th.FromRange(0, 50)
				out := make(chan int)

				go func() {
					universalLoop(ord, in, out, n, func(x int, canWrite <-chan struct{}) {
						res := x + 1000
						<-canWrite
						out <- res
					})
				}()

				outSlice := toSlice(out)

				expectedOutSlice := make([]int, 0, 50)
				for i := 0; i < 50; i++ {
					expectedOutSlice = append(expectedOutSlice, i+1000)
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expectedOutSlice)
			})

			t.Run(fmt.Sprint("concurrency_", n), func(t *testing.T) {
				in := th.FromRange(0, 2*n)
				out := make(chan int)

				var inProgress th.InProgressCounter

				go func() {
					OrderedLoop(in, out, n, func(x int, canWrite <-chan struct{}) {
						inProgress.Inc()
						time.Sleep(1 * time.Second)
						res := x + 1000
						inProgress.Dec()

						<-canWrite
						out <- res
					})
				}()

				drain(out)

				th.ExpectValue(t, inProgress.Max(), n)
			})

			t.Run(fmt.Sprint("ordering_", n), func(t *testing.T) {
				// use huge channel to minimize the chance of accidental ordering
				in := th.FromRange(0, 20000)
				out := make(chan int)

				go func() {
					universalLoop(ord, in, out, n, func(x int, canWrite <-chan struct{}) {
						res := x + 1000
						<-canWrite
						out <- res
					})
				}()

				outSlice := toSlice(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})
		}

	})
}

func TestBreak(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var in chan int
		in1, earlyExit := Break(in)
		th.ExpectValue(t, in1, nil)
		th.ExpectNotPanic(t, earlyExit)
	})

	t.Run("unbroken", func(t *testing.T) {
		in := th.FromRange(0, 10000)
		in1, _ := Break(in)

		maxSeen := -1

		for x := range in1 {
			if x > maxSeen {
				maxSeen = x
			}
		}

		th.ExpectValue(t, maxSeen, 9999)
		th.ExpectClosedChan(t, in, 1*time.Second)
	})

	t.Run("broken", func(t *testing.T) {
		in := th.FromRange(0, 1000)
		in1, earlyExit := Break(in)

		maxSeen := -1

		for x := range in1 {
			if x == 100 {
				earlyExit()
				time.Sleep(1 * time.Second) // give Break some time to react and drain
			}

			if x > maxSeen {
				maxSeen = x
			}
		}

		if maxSeen != 100 && maxSeen != 101 {
			// we can reach 101 because item #101 can be consumed by
			// the goroutine inside Break before earlyExit is called
			t.Errorf("expected 100 or 101, got %v", maxSeen)

		}

		th.ExpectClosedChan(t, in, 1*time.Second) // drained
	})

}

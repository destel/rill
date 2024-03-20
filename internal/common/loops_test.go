package common

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

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

func TestLoop(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {
			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				done := make(chan struct{})

				sum := int64(0)

				universalLoop(ord, in, done, n, func(x int, canWrite <-chan struct{}) {
					<-canWrite
					atomic.AddInt64(&sum, int64(x))
				})

				<-done
				th.ExpectValue(t, sum, 19*20/2)
			})

			t.Run(th.Name("concurrency and ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)
				out := make(chan int)

				var inProgress th.InProgressCounter

				universalLoop(ord, in, out, n, func(x int, canWrite <-chan struct{}) {
					inProgress.Inc()
					runtime.Gosched()
					inProgress.Dec()

					<-canWrite

					out <- x
				})

				outSlice := th.ToSlice(out)

				th.ExpectValue(t, inProgress.Max(), n)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}

			})
		}

	})
}

func TestBreakable(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var in chan int
		in1, earlyExit := Breakable(in)
		th.ExpectValue(t, in1, nil)
		th.ExpectNotPanic(t, earlyExit)
	})

	t.Run("normal", func(t *testing.T) {
		in := th.FromRange(0, 10000)
		in1, _ := Breakable(in)

		maxSeen := -1

		for x := range in1 {
			if x > maxSeen {
				maxSeen = x
			}
		}

		th.ExpectValue(t, maxSeen, 9999)
		th.ExpectDrainedChan(t, in)
	})

	t.Run("early exit", func(t *testing.T) {
		in := th.FromRange(0, 1000)
		in1, earlyExit := Breakable(in)

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

		th.ExpectDrainedChan(t, in)
	})

}

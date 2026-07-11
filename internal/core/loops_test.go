package core

import (
	"sync"
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
		th.TestLevels(t, []int{1, 3, 5}, func(t *testing.T, n int) {

			t.Run("nil", func(t *testing.T) {
				th.ExpectBlock(t, func(t *testing.T) {
					done := make(chan struct{})
					universalLoop(ord, nil, done, n, func(x int, canWrite <-chan struct{}) {
						<-canWrite
					})
					<-done
				})
			})

			th.RunSynctest(t, "correctness", func(t *testing.T) {
				in := th.FromRange(0, 20)
				done := make(chan struct{})

				var sum atomic.Int64

				universalLoop(ord, in, done, n, func(x int, canWrite <-chan struct{}) {
					th.SimulateWork(1*time.Second, 2*time.Second)
					<-canWrite
					sum.Add(int64(x))
				})

				<-done
				th.ExpectValue(t, sum.Load(), 19*20/2)
			})

			th.RunSynctest(t, "concurrency", func(t *testing.T) {
				in := th.FromRange(0, 100)
				out := make(chan int)

				var gauge th.InFlightGauge

				universalLoop(ord, in, out, n, func(x int, canWrite <-chan struct{}) {
					gauge.Enter()
					defer gauge.Exit()
					th.SimulateWork(1*time.Second, 2*time.Second)

					<-canWrite

					out <- x
				})

				Drain(out)

				th.ExpectValue(t, gauge.Max(), n)
			})

			th.RunSynctest(t, "ordering", func(t *testing.T) {
				in := th.FromRange(0, 100)
				out := make(chan int)

				universalLoop(ord, in, out, n, func(x int, canWrite <-chan struct{}) {
					th.SimulateWork(1*time.Second, 2*time.Second)
					if x%7 == 0 {
						time.Sleep(10 * time.Second) // force out-of-order completion
					}

					<-canWrite
					out <- x
				})

				outSlice := th.ToSlice(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		})
	})
}

func TestForEach(t *testing.T) {
	th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

		t.Run("nil", func(t *testing.T) {
			th.ExpectBlock(t, func(t *testing.T) {
				ForEach(nil, n, func(int) {})
			})
		})

		th.RunSynctest(t, "correctness", func(t *testing.T) {
			in := th.FromRange(0, 20)

			var sum atomic.Int64
			ForEach(in, n, func(x int) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				sum.Add(int64(x))
			})

			th.ExpectDrainedChan(t, in)

			th.ExpectValue(t, sum.Load(), 19*20/2)
		})

		th.RunSynctest(t, "concurrency", func(t *testing.T) {
			in := th.FromRange(0, 100)

			var gauge th.InFlightGauge

			ForEach(in, n, func(x int) {
				gauge.Enter()
				defer gauge.Exit()
				th.SimulateWork(1*time.Second, 2*time.Second)
			})

			th.ExpectValue(t, gauge.Max(), n)
		})

		th.RunSynctest(t, "ordering", func(t *testing.T) {
			in := th.FromRange(0, 100)

			outSlice := make([]int, 0, 1000)
			var mu sync.Mutex

			ForEach(in, n, func(x int) {
				th.SimulateWork(1*time.Second, 2*time.Second)
				if x%7 == 0 {
					time.Sleep(10 * time.Second) // force out-of-order completion
				}

				mu.Lock()
				outSlice = append(outSlice, x)
				mu.Unlock()
			})

			if n == 1 {
				th.ExpectSorted(t, outSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
			}
		})

	})
}

package core

import (
	"sync/atomic"
	"testing"

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
		for _, n := range []int{1, 3, 5} {
			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				done := make(chan struct{})

				var sum atomic.Int64

				universalLoop(ord, in, done, n, func(x int, canWrite <-chan struct{}) {
					<-canWrite
					sum.Add(int64(x))
				})

				<-done
				th.ExpectValue(t, sum.Load(), 19*20/2)
			})

			th.RunSynctest(t, th.Name("concurrency", n), func(t *testing.T) {
				in := th.FromRange(0, 100)
				out := make(chan int)

				var monitor th.ConcurrencyMonitor

				universalLoop(ord, in, out, n, func(x int, canWrite <-chan struct{}) {
					monitor.Enter()
					defer monitor.Exit()

					<-canWrite

					out <- x
				})

				Drain(out)

				th.ExpectValue(t, monitor.Max(), n)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)
				out := make(chan int)

				universalLoop(ord, in, out, n, func(x int, canWrite <-chan struct{}) {

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
		}

	})
}

func TestForEach(t *testing.T) {
	for _, n := range []int{1, 5} {
		t.Run(th.Name("nil", n), func(t *testing.T) {
			th.ExpectDeadlock(t, func() {
				ForEach(nil, n, func(int) {})
			})
		})

		t.Run(th.Name("correctness", n), func(t *testing.T) {
			in := th.FromRange(0, 20)

			var sum atomic.Int64

			ForEach(in, n, func(x int) {
				sum.Add(int64(x))
			})

			th.ExpectValue(t, sum.Load(), 19*20/2)
		})

		th.RunSynctest(t, th.Name("concurrency", n), func(t *testing.T) {
			in := th.FromRange(0, 100)

			var monitor th.ConcurrencyMonitor

			ForEach(in, n, func(x int) {
				monitor.Enter()
				defer monitor.Exit()
			})

			th.ExpectValue(t, monitor.Max(), n)
		})

		t.Run(th.Name("ordering", n), func(t *testing.T) {
			in := th.FromRange(0, 20000)
			out := make(chan int)

			go func() {
				ForEach(in, n, func(x int) {
					out <- x
				})
				close(out)
			}()

			outSlice := th.ToSlice(out)

			if n == 1 {
				th.ExpectSorted(t, outSlice)
			} else {
				th.ExpectUnsorted(t, outSlice)
			}
		})
	}
}

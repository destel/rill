package rill

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

const benchmarkInputSize = 100000

// code called on each benchmark iteration
func benchmarkIteration() {
	busySleep(1 * time.Microsecond)
	//time.Sleep(1 * time.Microsecond)
	//busySleep(10 * time.Microsecond)
	//time.Sleep(10 * time.Microsecond)
}

func busySleep(d time.Duration) {
	if d == 0 {
		return
	}

	start := time.Now()
	for time.Since(start) < d {
	}
}

func runBenchmark[B any](b *testing.B, name string, body func(in <-chan Try[int]) <-chan B) {
	b.Run(name, func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()

			in := make(chan Try[int])
			done := make(chan struct{})

			go func() {
				defer close(done)
				out := body(in)

				if out != nil {
					Drain(out)
				}
			}()

			// Give body a some time to spawn goroutines
			time.Sleep(100 * time.Millisecond)

			b.StartTimer()

			// write to input
			for k := 0; k < benchmarkInputSize; k++ {
				in <- Try[int]{Value: k}
			}
			close(in)

			// wait for body to finish
			<-done
			b.StopTimer()
		}
	})
}

// Benchmarks below are commented out to remove dependency on errgroup

//// This benchmark uses classic goroutine-per-item + semaphore pattern.
//func BenchmarkErrGroupWithSetLimit(b *testing.B) {
//	for _, n := range []int{1, 2, 4, 8} {
//		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
//			var eg errgroup.Group
//			eg.SetLimit(n)
//
//			for x := range in {
//				x := x
//				eg.Go(func() error {
//					if err := x.Error; err != nil {
//						return err
//					}
//					benchmarkIteration()
//					return nil
//				})
//			}
//
//			_ = eg.Wait()
//			return nil
//		})
//	}
//}
//
//// This benchmark uses much less common pattern of creating a worker pool with errgroup.
//func BenchmarkErrGroupWithWorkerPool(b *testing.B) {
//	for _, n := range []int{1, 2, 4, 8} {
//		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
//			var eg errgroup.Group
//			for i := 0; i < n; i++ {
//				eg.Go(func() error {
//					for x := range in {
//						if err := x.Error; err != nil {
//							return err
//						}
//						benchmarkIteration()
//					}
//					return nil
//				})
//			}
//			_ = eg.Wait()
//			return nil
//		})
//	}
//}

func BenchmarkForEach(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
			_ = ForEach(in, n, func(x int) error {
				benchmarkIteration()
				return nil
			})
			return nil
		})
	}
}

func BenchmarkMapAndDrain(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
			return Map(in, n, func(x int) (int, error) {
				benchmarkIteration()
				return x, nil
			})
		})
	}
}

func BenchmarkReduce(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan int {
			_, _, _ = Reduce(in, n, func(x, y int) (int, error) {
				benchmarkIteration()
				return x, nil
			})
			return nil
		})
	}
}

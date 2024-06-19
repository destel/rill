package rill

import (
	"sync"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

const benchmarkInputSize = 100000
const benchmarkWorkDuration = 10 * time.Microsecond

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

func busySleep(d time.Duration) {
	if d == 0 {
		return
	}

	start := time.Now()
	for time.Since(start) < d {
	}
}

func BenchmarkBasicForLoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for k := 0; k < benchmarkInputSize; k++ {
			busySleep(benchmarkWorkDuration)
		}
	}
}

func BenchmarkBasicForLoopWithSleep(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for k := 0; k < benchmarkInputSize; k++ {
			time.Sleep(benchmarkWorkDuration)
		}
	}
}

func BenchmarkWaitGroup(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
			var wg sync.WaitGroup
			wg.Add(n)
			for i := 0; i < n; i++ {
				go func() {
					defer wg.Done()
					for range in {
						busySleep(benchmarkWorkDuration)
					}
				}()
			}
			wg.Wait()
			return nil
		})
	}
}

func BenchmarkWaitGroupWithSleep(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
			var wg sync.WaitGroup
			wg.Add(n)
			for i := 0; i < n; i++ {
				go func() {
					defer wg.Done()
					for range in {
						time.Sleep(benchmarkWorkDuration)
					}
				}()
			}
			wg.Wait()
			return nil
		})
	}
}

func BenchmarkForEach(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
			ForEach(in, n, func(x int) error {
				busySleep(benchmarkWorkDuration)
				return nil
			})
			return nil
		})
	}
}

func BenchmarkForEachWithSleep(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
			ForEach(in, n, func(x int) error {
				time.Sleep(benchmarkWorkDuration)
				return nil
			})
			return nil
		})
	}
}

func BenchmarkMap(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
			return Map(in, n, func(x int) (int, error) {
				busySleep(benchmarkWorkDuration)
				return x, nil
			})
		})
	}
}

func BenchmarkMapWithSleep(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan Try[int] {
			return Map(in, n, func(x int) (int, error) {
				time.Sleep(benchmarkWorkDuration)
				return x, nil
			})
		})
	}
}

func BenchmarkReduce(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan int {
			Reduce(in, n, func(x, y int) (int, error) {
				busySleep(benchmarkWorkDuration)
				return x, nil
			})
			return nil
		})
	}
}

func BenchmarkReduceWithSleep(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		runBenchmark(b, th.Name(n), func(in <-chan Try[int]) <-chan int {
			Reduce(in, n, func(x, y int) (int, error) {
				time.Sleep(benchmarkWorkDuration)
				return x, nil
			})
			return nil
		})
	}
}

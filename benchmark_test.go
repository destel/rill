package rill

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

// The stock B/op and allocs/op columns are the per-item metrics reported by
// benchmarkThroughput, integer divided, so anything under one allocation per
// item reads as zero. We use custom floating point metrics and suppress the
// stock ones - even when -benchmem is passed explicitly.
// This can only be done by mutating the flag.
func TestMain(m *testing.M) {
	flag.Parse()
	flag.Lookup("test.benchmem").Value.Set("false") //nolint:errcheck
	os.Exit(m.Run())
}

// benchmarkThroughput feeds b.N items into the pipeline from a single
// goroutine and waits for it to finish, so ns/op is the per-item cost
// including the pipeline's tail work after the input is exhausted.
func benchmarkThroughput(b *testing.B, definePipeline func(in <-chan Try[int])) {
	in := make(chan Try[int])
	done := make(chan struct{})

	go func() {
		defer close(done)
		definePipeline(in)
	}()

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		in <- Try[int]{Value: i}
	}

	close(in)
	<-done

	b.StopTimer()
	runtime.ReadMemStats(&after)

	// Report custom floating point metrics.
	b.ReportMetric(float64(after.Mallocs-before.Mallocs)/float64(b.N), "allocs/item")
	b.ReportMetric(float64(after.TotalAlloc-before.TotalAlloc)/float64(b.N), "B/item")
}

// benchmarkThroughputForLevels runs benchmarkThroughput once per concurrency
// level, as a subtest named name/n.
func benchmarkThroughputForLevels(b *testing.B, name string, levels []int, definePipeline func(in <-chan Try[int], n int)) {
	for _, n := range levels {
		b.Run(fmt.Sprintf("%s/%d", name, n), func(b *testing.B) {
			benchmarkThroughput(b, func(in <-chan Try[int]) {
				definePipeline(in, n)
			})
		})
	}
}

// This benchmark acts as baseline. A single drainer is
// as simple as pipeline can get.
func BenchmarkDrain(b *testing.B) {
	benchmarkThroughput(b, func(in <-chan Try[int]) {
		Drain(in)
	})
}

// Benchmarking errgroup.Group as a baseline for ForEach.
// Commented out to keep rill dependency-free. To run, add golang.org/x/sync
// to go.mod and uncomment the block below

// func BenchmarkErrGroup(b *testing.B) {
// 	levels := []int{1, 2, 4, 8, 32}
//
// 	// goroutine per item, bounded by a semaphore
// 	setLimit := func(in <-chan Try[int], n int, f func(int) error) {
// 		var eg errgroup.Group
// 		eg.SetLimit(n)
// 		for x := range in {
// 			if x.Error != nil {
// 				break
// 			}
// 			eg.Go(func() error { return f(x.Value) })
// 		}
// 		_ = eg.Wait()
// 	}
//
// 	// long-lived workers, the shape ForEach itself uses
// 	pool := func(in <-chan Try[int], n int, f func(int) error) {
// 		var eg errgroup.Group
// 		for range n {
// 			eg.Go(func() error {
// 				for x := range in {
// 					if x.Error != nil {
// 						return x.Error
// 					}
// 					if err := f(x.Value); err != nil {
// 						return err
// 					}
// 				}
// 				return nil
// 			})
// 		}
// 		_ = eg.Wait()
// 	}
//
// 	benchmarkThroughputForLevels(b, "noop/setlimit", levels, func(in <-chan Try[int], n int) {
// 		setLimit(in, n, func(x int) error {
// 			return nil
// 		})
// 	})
//
// 	benchmarkThroughputForLevels(b, "noop/pool", levels, func(in <-chan Try[int], n int) {
// 		pool(in, n, func(x int) error {
// 			return nil
// 		})
// 	})
//
// 	benchmarkThroughputForLevels(b, "50us/setlimit", levels, func(in <-chan Try[int], n int) {
// 		setLimit(in, n, func(x int) error {
// 			time.Sleep(50 * time.Microsecond)
// 			return nil
// 		})
// 	})
//
// 	benchmarkThroughputForLevels(b, "50us/pool", levels, func(in <-chan Try[int], n int) {
// 		pool(in, n, func(x int) error {
// 			time.Sleep(50 * time.Microsecond)
// 			return nil
// 		})
// 	})
// }

func BenchmarkForEach(b *testing.B) {
	levels := []int{1, 2, 4, 8, 32}

	benchmarkThroughputForLevels(b, "noop", levels, func(in <-chan Try[int], n int) {
		_ = ForEach(in, n, func(x int) error {
			return nil
		})
	})

	benchmarkThroughputForLevels(b, "50us", levels, func(in <-chan Try[int], n int) {
		_ = ForEach(in, n, func(x int) error {
			time.Sleep(50 * time.Microsecond)
			return nil
		})
	})
}

func BenchmarkMap(b *testing.B) {
	levels := []int{1, 2, 4, 8, 32}

	benchmarkThroughputForLevels(b, "noop", levels, func(in <-chan Try[int], n int) {
		out := Map(in, n, func(x int) (int, error) {
			return x, nil
		})
		Drain(out)
	})

	benchmarkThroughputForLevels(b, "50us", levels, func(in <-chan Try[int], n int) {
		out := Map(in, n, func(x int) (int, error) {
			time.Sleep(50 * time.Microsecond)
			return x, nil
		})
		Drain(out)
	})
}

func BenchmarkOrderedMap(b *testing.B) {
	levels := []int{1, 2, 4, 8, 32}

	benchmarkThroughputForLevels(b, "noop", levels, func(in <-chan Try[int], n int) {
		out := OrderedMap(in, n, func(x int) (int, error) {
			return x, nil
		})
		Drain(out)
	})

	benchmarkThroughputForLevels(b, "50us", levels, func(in <-chan Try[int], n int) {
		out := OrderedMap(in, n, func(x int) (int, error) {
			time.Sleep(50 * time.Microsecond)
			return x, nil
		})
		Drain(out)
	})
}

// Cost per item in the absorb and rebuild workloads depends on the input size,
// so it's better to compare performance at fixed input size (e.g. -benchtime=1000x).
// Try modest sizes first, since rebuild at n=1 is quadratic.
func BenchmarkReduce(b *testing.B) {
	levels := []int{1, 2, 4, 8, 32}

	benchmarkThroughputForLevels(b, "noop", levels, func(in <-chan Try[int], n int) {
		_, _, _ = Reduce(in, n, func(x, y int) (int, error) {
			return 0, nil
		})
	})

	benchmarkThroughputForLevels(b, "50us", levels, func(in <-chan Try[int], n int) {
		_, _, _ = Reduce(in, n, func(x, y int) (int, error) {
			time.Sleep(50 * time.Microsecond)
			return 0, nil
		})
	})

	// Workloads below price a merge by the number of items it covers.
	// We use negative numbers to encode a span of items, while positive numbers
	// encode a single item coming from the pipeline.
	spanSize := func(v int) int {
		if v < 0 {
			return -v
		}
		return 1
	}

	benchmarkThroughputForLevels(b, "absorb", levels, func(in <-chan Try[int], n int) {
		_, _, _ = Reduce(in, n, func(x, y int) (int, error) {
			// operation cost is proportional to the smaller of the two operands:
			// simulates workloads where the larger operand absorbs the smaller one.
			sx, sy := spanSize(x), spanSize(y)
			time.Sleep(time.Duration(min(sx, sy)) * 10 * time.Microsecond)
			return -(sx + sy), nil
		})
	})

	benchmarkThroughputForLevels(b, "rebuild", levels, func(in <-chan Try[int], n int) {
		_, _, _ = Reduce(in, n, func(x, y int) (int, error) {
			// operation cost is proportional to the sum of the two operands:
			// simulates workloads where the two operands are "concatenated".
			sx, sy := spanSize(x), spanSize(y)
			time.Sleep(time.Duration(sx+sy) * 10 * time.Microsecond)
			return -(sx + sy), nil
		})
	})
}

func BenchmarkMapReduce(b *testing.B) {
	mapperLevels := []int{1, 2, 4}
	reducerLevels := []int{1, 2, 4, 8, 32}

	runForLevels := func(b *testing.B, name string, definePipeline func(in <-chan Try[int], nm, nr int)) {
		for _, nm := range mapperLevels {
			for _, nr := range reducerLevels {
				b.Run(fmt.Sprintf("%s/%d-%d", name, nm, nr), func(b *testing.B) {
					benchmarkThroughput(b, func(in <-chan Try[int]) {
						definePipeline(in, nm, nr)
					})
				})
			}
		}
	}

	runForLevels(b, "noop-low-card", func(in <-chan Try[int], nm, nr int) {
		_, _ = MapReduce(in, nm, func(x int) (int, int, error) {
			return x % 10, 0, nil
		}, nr, func(x, y int) (int, error) {
			return 0, nil
		})
	})

	runForLevels(b, "50us-low-card", func(in <-chan Try[int], nm, nr int) {
		_, _ = MapReduce(in, nm, func(x int) (int, int, error) {
			return x % 10, 0, nil
		}, nr, func(x, y int) (int, error) {
			time.Sleep(50 * time.Microsecond)
			return 0, nil
		})
	})

	runForLevels(b, "noop-high-card", func(in <-chan Try[int], nm, nr int) {
		_, _ = MapReduce(in, nm, func(x int) (int, int, error) {
			return x % 10000, 0, nil
		}, nr, func(x, y int) (int, error) {
			return 0, nil
		})
	})

	runForLevels(b, "50us-high-card", func(in <-chan Try[int], nm, nr int) {
		_, _ = MapReduce(in, nm, func(x int) (int, int, error) {
			return x % 10000, 0, nil
		}, nr, func(x, y int) (int, error) {
			time.Sleep(50 * time.Microsecond)
			return 0, nil
		})
	})
}

package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func universalMapOrFilter[A, B any](ord bool, in <-chan A, n int, f func(A) (B, bool)) <-chan B {
	if ord {
		return OrderedMapOrFilter(in, n, f)
	}
	return MapOrFilter(in, n, f)
}

func TestMapOrFilter(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalMapOrFilter(ord, nil, n, func(x int) (int, bool) { return x, true })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := universalMapOrFilter(ord, in, n, func(x int) (string, bool) {
					return fmt.Sprintf("%03d", x), x%2 == 0
				})

				outSlice := toSlice(out)

				expectedSlice := make([]string, 0, 20)
				for i := 0; i < 20; i++ {
					if i%2 != 0 {
						continue
					}
					expectedSlice = append(expectedSlice, fmt.Sprintf("%03d", i))
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expectedSlice)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				out := universalMapOrFilter(ord, in, n, func(x int) (int, bool) {
					return x, x%2 == 0
				})

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

func universalMapOrFlatMap[A, B any](ord bool, in <-chan A, n int, f func(A) (b B, bb <-chan B, flat bool)) <-chan B {
	if ord {
		return OrderedMapOrFlatMap(in, n, f)
	}
	return MapOrFlatMap(in, n, f)
}

func TestMapOrFlatMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalMapOrFlatMap(ord, nil, n, func(x int) (int, <-chan int, bool) { return x, nil, true })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := universalMapOrFlatMap(ord, in, n, func(x int) (string, <-chan string, bool) {
					if x%2 == 0 {
						return fmt.Sprintf("%03dA", x), nil, false
					} else {
						res := fromSlice([]string{fmt.Sprintf("%03dB", x), fmt.Sprintf("%03dC", x)})
						return "dummy", res, true
					}
				})

				outSlice := toSlice(out)

				expectedSlice := make([]string, 0, 2*20)
				for i := 0; i < 20; i++ {
					if i%2 == 0 {
						expectedSlice = append(expectedSlice, fmt.Sprintf("%03dA", i))
					} else {
						expectedSlice = append(expectedSlice, fmt.Sprintf("%03dB", i), fmt.Sprintf("%03dC", i))
					}
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expectedSlice)
			})

			t.Run(th.Name("nil hang", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := universalMapOrFlatMap(ord, in, n, func(x int) (string, <-chan string, bool) {
					if x == 10 {
						return "", nil, true
					} else {
						return "", fromSlice([]string{"B", "C"}), true
					}
				})

				th.ExpectNeverClosedChan(t, out, 1*time.Second)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				out := universalMapOrFlatMap(ord, in, n, func(x int) (string, <-chan string, bool) {
					if x%2 == 0 {
						return fmt.Sprintf("%06dA", x), nil, false
					} else {
						res := fromSlice([]string{
							fmt.Sprintf("%06dB", x),
							fmt.Sprintf("%06dC", x),
							fmt.Sprintf("%06dD", x),
						})
						return "", res, true
					}
				})

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

func universalMapAndSplit[A, B any](ord bool, in <-chan A, numOuts int, n int, f func(A) (B, int)) []<-chan B {
	if ord {
		return OrderedMapAndSplit(in, numOuts, n, f)
	}
	return MapAndSplit(in, numOuts, n, f)
}

func TestOrderedMapAndSplit(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {
			for _, numOuts := range []int{3} {

				t.Run(th.Name("nil", numOuts, n), func(t *testing.T) {
					outs := universalMapAndSplit(ord, nil, numOuts, n, func(x int) (int, int) { return x, 0 })
					th.ExpectSlice(t, outs, make([]<-chan int, numOuts))
				})

				t.Run(th.Name("correctness", numOuts, n), func(t *testing.T) {
					in := th.FromRange(0, 20*(numOuts+1))
					outs := universalMapAndSplit(ord, in, numOuts, n, func(x int) (string, int) {
						outId := x % (numOuts + 1)
						return fmt.Sprintf("%03d", x), outId
					})

					outSlices := make([][]string, numOuts)
					th.DoConcurrentlyN(numOuts, func(i int) {
						outSlices[i] = toSlice(outs[i])
					})

					expectedSlices := make([][]string, 3)
					for i := 0; i < 20*(numOuts+1); i++ {
						outId := i % (numOuts + 1)
						if outId >= numOuts {
							continue
						}

						expectedSlices[outId] = append(expectedSlices[outId], fmt.Sprintf("%03d", i))
					}

					for i := range outSlices {
						th.Sort(outSlices[i])
						th.ExpectSlice(t, outSlices[i], expectedSlices[i])
					}
				})

				t.Run(th.Name("ordering", numOuts, n), func(t *testing.T) {
					in := th.FromRange(0, 10000*(numOuts+1))

					outs := universalMapAndSplit(ord, in, numOuts, n, func(x int) (string, int) {
						outId := x % (numOuts + 1)
						return fmt.Sprintf("%06d", x), outId
					})

					outSlices := make([][]string, numOuts)
					th.DoConcurrentlyN(len(outs), func(i int) {
						outSlices[i] = toSlice(outs[i])
					})

					for i := range outSlices {
						if ord || n == 1 {
							th.ExpectSorted(t, outSlices[i])
						} else {
							th.ExpectUnsorted(t, outSlices[i])
						}
					}
				})

			}
		}
	})
}

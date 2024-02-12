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

func universalMapAndSplit[A, B any](ord bool, in <-chan A, n int, numOuts int, f func(A) (B, int)) []<-chan B {
	if ord {
		return OrderedMapAndSplit(in, n, numOuts, f)
	}
	return MapAndSplit(in, n, numOuts, f)
}

func TestOrderedMapAndSplit(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				outs := universalMapAndSplit(ord, nil, n, 3, func(x int) (int, int) { return x, 0 })
				th.ExpectSlice(t, outs, []<-chan int{nil, nil, nil})
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				outs := universalMapAndSplit(ord, in, n, 3, func(x int) (string, int) {
					switch x % 4 {
					case 0:
						return fmt.Sprintf("%03dA", x), 0
					case 1:
						return fmt.Sprintf("%03dB", x), 1
					case 2:
						return fmt.Sprintf("%03dC", x), 2
					default:
						return "", -1 // discard
					}
				})

				outSlices := make([][]string, len(outs))
				th.DoConcurrentlyN(len(outs), func(i int) {
					outSlices[i] = toSlice(outs[i])
				})

				expectedSlices := make([][]string, 3)
				for i := 0; i < 20; i++ {
					switch i % 4 {
					case 0:
						expectedSlices[0] = append(expectedSlices[0], fmt.Sprintf("%03dA", i))
					case 1:
						expectedSlices[1] = append(expectedSlices[1], fmt.Sprintf("%03dB", i))
					case 2:
						expectedSlices[2] = append(expectedSlices[2], fmt.Sprintf("%03dC", i))
					}
				}

				for i := range outs {
					th.Sort(outSlices[i])
					th.ExpectSlice(t, outSlices[i], expectedSlices[i])
				}
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				outs := universalMapAndSplit(ord, in, n, 3, func(x int) (string, int) {
					switch x % 4 {
					case 0:
						return fmt.Sprintf("%06dA", x), 0
					case 1:
						return fmt.Sprintf("%06dB", x), 1
					case 2:
						return fmt.Sprintf("%06dC", x), 2
					default:
						return "", -1 // discard
					}
				})

				outSlices := make([][]string, len(outs))
				th.DoConcurrentlyN(len(outs), func(i int) {
					outSlices[i] = toSlice(outs[i])
				})

				for i := range outs {
					if ord || n == 1 {
						th.ExpectSorted(t, outSlices[i])
					} else {
						th.ExpectUnsorted(t, outSlices[i])
					}
				}
			})

		}
	})
}

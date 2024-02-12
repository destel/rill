package chans

import (
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestMerge(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := Merge[string]()
		th.ExpectValue(t, out, nil)
	})

	for _, numChans := range []int{1, 3, 5, 10} {
		t.Run(th.Name("correctness", numChans), func(t *testing.T) {
			ins := make([]<-chan int, numChans)

			for i := 0; i < numChans; i++ {
				ins[i] = th.FromRange(i*10, (i+1)*10)
			}

			out := Merge(ins...)
			outSlice := ToSlice(out)

			expectedSlice := make([]int, 0, numChans*10)
			for i := 0; i < numChans*10; i++ {
				expectedSlice = append(expectedSlice, i)
			}

			th.Sort(outSlice)
			th.ExpectSlice(t, outSlice, expectedSlice)
		})

		t.Run(th.Name("nil hang", numChans), func(t *testing.T) {
			ins := make([]<-chan int, numChans)

			for i := 0; i < numChans-1; i++ {
				ins[i] = th.FromRange(i*10, (i+1)*10)
			}

			// make last channel nil
			ins[numChans-1] = nil

			out := Merge(ins...)

			th.ExpectNeverClosedChan(t, out, 1*time.Second)
		})

	}
}

func universalSplit[A any](ord bool, in <-chan A, numOuts int, n int, f func(A) int) []<-chan A {
	switch {
	case !ord && numOuts == 2:
		out0, out1 := Split2(in, n, f)
		return []<-chan A{out0, out1}
	case ord && numOuts == 2:
		out0, out1 := OrderedSplit2(in, n, f)
		return []<-chan A{out0, out1}
	default:
		panic("unsupported")
	}
}

func TestSplit2(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {
			for _, numOuts := range []int{2} {

				t.Run(th.Name("nil", numOuts, n), func(t *testing.T) {
					outs := universalSplit(ord, nil, numOuts, n, func(x int) int { return 0 })
					th.ExpectSlice(t, outs, make([]<-chan int, numOuts))
				})

				t.Run(th.Name("correctness", numOuts, n), func(t *testing.T) {
					// idea: split input into numOuts+1 groups
					// - first numOuts groups are sent into corresponding outputs
					// - next group is filtered out

					in := th.FromRange(0, 20*(numOuts+1))
					outs := universalSplit(ord, in, numOuts, n, func(x int) int {
						return x % (numOuts + 1)
					})

					outSlices := make([][]int, numOuts)
					th.DoConcurrentlyN(numOuts, func(i int) {
						outSlices[i] = ToSlice(outs[i])
					})

					expectedSlices := make([][]int, 3)
					for i := 0; i < 20*(numOuts+1); i++ {
						outID := i % (numOuts + 1)
						if outID >= numOuts {
							continue
						}

						expectedSlices[outID] = append(expectedSlices[outID], i)
					}

					for i := range outSlices {
						th.Sort(outSlices[i])
						th.ExpectSlice(t, outSlices[i], expectedSlices[i])
					}
				})

				t.Run(th.Name("ordering", numOuts, n), func(t *testing.T) {
					in := th.FromRange(0, 10000*numOuts)

					outs := universalSplit(ord, in, numOuts, n, func(x int) int {
						return x % numOuts
					})

					outSlices := make([][]int, numOuts)
					th.DoConcurrentlyN(len(outs), func(i int) {
						outSlices[i] = ToSlice(outs[i])
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

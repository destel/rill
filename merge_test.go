package rill

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestMerge(t *testing.T) {
	// real tests are in another package
	Merge[int](nil)
}

func universalSplit[A any](ord bool, in <-chan Try[A], numOuts int, n int, f func(A) (int, error)) []<-chan Try[A] {
	switch {
	case !ord && numOuts == 2:
		out0, out1 := Split2(in, n, f)
		return []<-chan Try[A]{out0, out1}
	case ord && numOuts == 2:
		out0, out1 := OrderedSplit2(in, n, f)
		return []<-chan Try[A]{out0, out1}
	default:
		panic("unsupported")
	}
}

func TestSplit2(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {
			for _, numOuts := range []int{2} {

				t.Run(th.Name("nil", numOuts, n), func(t *testing.T) {
					outs := universalSplit(ord, nil, numOuts, n, func(string) (int, error) { return 0, nil })
					th.ExpectSlice(t, outs, make([]<-chan Try[string], numOuts))
				})

				t.Run(th.Name("correctness", numOuts, n), func(t *testing.T) {
					// idea: split input into numOuts+3 groups
					// - first numOuts groups are sent into corresponding outputs
					// - next group is filtered out
					// - next group would cause error during splitting
					// - next group would be errors even before splitting

					in := WrapChan(th.FromRange(0, 20*(numOuts+3)), nil)
					in = OrderedMap(in, 1, func(x int) (int, error) {
						outID := x % (numOuts + 3)
						if outID == numOuts+2 {
							return x, fmt.Errorf("err%03d", x)
						}
						return x, nil
					})

					outs := universalSplit(ord, in, numOuts, n, func(x int) (int, error) {
						outID := x % (numOuts + 3)
						if outID == numOuts+1 {
							return 0, fmt.Errorf("err%03d", x)
						}
						return outID, nil
					})

					outSlices := make([][]int, numOuts)
					errSlices := make([][]string, numOuts)

					th.DoConcurrentlyN(numOuts, func(i int) {
						outSlices[i], errSlices[i] = toSliceAndErrors(outs[i])
					})

					expectedOutSlices := make([][]int, numOuts)
					expectedAllErrsSlice := make([]string, 0, numOuts)

					for i := 0; i < 20*(numOuts+3); i++ {
						outID := i % (numOuts + 3)
						if outID > numOuts {
							expectedAllErrsSlice = append(expectedAllErrsSlice, fmt.Sprintf("err%03d", i))
						}
						if outID < numOuts {
							expectedOutSlices[outID] = append(expectedOutSlices[outID], i)
						}
					}

					var allErrsSlice []string
					for _, errSlice := range errSlices {
						allErrsSlice = append(allErrsSlice, errSlice...)
						if len(errSlice) == 0 {
							t.Errorf("expected at least one error in each channel")
						}
					}

					th.Sort(allErrsSlice)
					th.ExpectSlice(t, allErrsSlice, expectedAllErrsSlice)

					for i := range outSlices {
						th.Sort(outSlices[i])
						th.ExpectSlice(t, outSlices[i], expectedOutSlices[i])
					}

				})

				t.Run(th.Name("ordering", numOuts, n), func(t *testing.T) {
					in := WrapChan(th.FromRange(0, 10000*(numOuts+1)), nil)

					outs := universalSplit(ord, in, numOuts, n, func(x int) (int, error) {
						outID := x % (numOuts + 1)
						if outID == numOuts {
							return 0, fmt.Errorf("err%06d", x)
						}
						return outID, nil
					})

					outSlices := make([][]int, numOuts)
					errSlices := make([][]string, numOuts)

					th.DoConcurrentlyN(len(outs), func(i int) {
						outSlices[i], errSlices[i] = toSliceAndErrors(outs[i])
					})

					for i := range outSlices {
						if ord || n == 1 {
							th.ExpectSorted(t, outSlices[i])
							th.ExpectSorted(t, errSlices[i])
						} else {
							th.ExpectUnsorted(t, outSlices[i])
							th.ExpectUnsorted(t, errSlices[i])
						}
					}
				})

			}
		}
	})
}

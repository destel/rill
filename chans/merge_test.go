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

			timer := time.After(1 * time.Second)
			for {
				select {
				case _, ok := <-out:
					if !ok {
						t.Errorf("hang expected, but channel was closed")
					}

				case <-timer:
					return
				}

			}

		})

	}
}

func universalSplit2[A any](ord bool, in <-chan A, n int, f func(A) bool) (<-chan A, <-chan A) {
	if ord {
		return OrderedSplit2(in, n, f)
	}
	return Split2(in, n, f)
}

func TestSplit2(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				outT, outF := universalSplit2(ord, nil, n, func(x int) bool { return true })
				th.ExpectValue(t, outT, nil)
				th.ExpectValue(t, outF, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				outT, outF := universalSplit2(ord, in, n, func(x int) bool {
					return x%3 == 0
				})

				expectedSliceT := make([]int, 0, 20)
				expectedSliceF := make([]int, 0, 20)
				for i := 0; i < 20; i++ {
					if i%3 == 0 {
						expectedSliceT = append(expectedSliceT, i)
					} else {
						expectedSliceF = append(expectedSliceF, i)
					}
				}

				var outSliceT, outSliceF []int

				th.DoConcurrently(
					func() { outSliceT = ToSlice(outT) },
					func() { outSliceF = ToSlice(outF) },
				)

				th.Sort(outSliceT)
				th.Sort(outSliceF)

				th.ExpectSlice(t, outSliceT, expectedSliceT)
				th.ExpectSlice(t, outSliceF, expectedSliceF)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				outT, outF := universalSplit2(ord, in, n, func(x int) bool {
					return x%2 == 0
				})

				var outSliceT, outSliceF []int

				th.DoConcurrently(
					func() { outSliceT = ToSlice(outT) },
					func() { outSliceF = ToSlice(outF) },
				)

				if ord || n == 1 {
					th.ExpectSorted(t, outSliceT)
					th.ExpectSorted(t, outSliceF)
				} else {
					th.ExpectUnsorted(t, outSliceT)
					th.ExpectUnsorted(t, outSliceF)
				}
			})

		}
	})
}

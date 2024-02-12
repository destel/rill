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

func universalSplit2[A any](ord bool, in <-chan A, n int, f func(A) int) (<-chan A, <-chan A) {
	if ord {
		return OrderedSplit2(in, n, f)
	}
	return Split2(in, n, f)
}

func TestSplit2(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out0, out1 := universalSplit2(ord, nil, n, func(x int) int { return 0 })
				th.ExpectValue(t, out0, nil)
				th.ExpectValue(t, out1, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out0, out1 := universalSplit2(ord, in, n, func(x int) int {
					return x % 3
				})

				expectedSlice0 := make([]int, 0, 20)
				expectedSlice1 := make([]int, 0, 20)
				for i := 0; i < 20; i++ {
					switch i % 3 {
					case 0:
						expectedSlice0 = append(expectedSlice0, i)
					case 1:
						expectedSlice1 = append(expectedSlice1, i)
					}
				}

				var outSlice0, outSlice1 []int

				th.DoConcurrently(
					func() { outSlice0 = ToSlice(out0) },
					func() { outSlice1 = ToSlice(out1) },
				)

				th.Sort(outSlice0)
				th.Sort(outSlice1)

				th.ExpectSlice(t, outSlice0, expectedSlice0)
				th.ExpectSlice(t, outSlice1, expectedSlice1)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				out0, out1 := universalSplit2(ord, in, n, func(x int) int {
					return x % 2
				})

				var outSlice0, outSlice1 []int

				th.DoConcurrently(
					func() { outSlice0 = ToSlice(out0) },
					func() { outSlice1 = ToSlice(out1) },
				)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice0)
					th.ExpectSorted(t, outSlice1)
				} else {
					th.ExpectUnsorted(t, outSlice0)
					th.ExpectUnsorted(t, outSlice1)
				}
			})

		}
	})
}

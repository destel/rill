package echans

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func universalSplit2[A any](ord bool, in <-chan Try[A], n int, f func(A) (int, error)) (<-chan Try[A], <-chan Try[A]) {
	if ord {
		return OrderedSplit2(in, n, f)
	}
	return Split2(in, n, f)
}

func TestSplit2(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out0, out1 := universalSplit2(ord, nil, 5, func(string) (int, error) { return 0, nil })
				th.ExpectValue(t, out0, nil)
				th.ExpectValue(t, out1, nil)

			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 200), nil)
				in = OrderedMap(in, 1, func(x int) (int, error) {
					if x%5 == 4 {
						return 0, fmt.Errorf("err%03d", x)
					}
					return x, nil
				})

				outT, outF := universalSplit2(ord, in, n, func(x int) (int, error) {
					if x%5 == 3 {
						return 0, fmt.Errorf("err%03d", x)
					}
					return x % 5, nil
				})

				expectedSlice0 := make([]int, 0, 200)
				expectedSlice1 := make([]int, 0, 200)
				expectedAllErrsSLice := make([]string, 0, 200)

				for i := 0; i < 200; i++ {
					switch i % 5 {
					case 0:
						expectedSlice0 = append(expectedSlice0, i)
					case 1:
						expectedSlice1 = append(expectedSlice1, i)
					case 3, 4:
						expectedAllErrsSLice = append(expectedAllErrsSLice, fmt.Sprintf("err%03d", i))
					}
				}

				var outSlice0, outSlice1 []int
				var errSlice0, errSlice1, allErrsSlice []string

				th.DoConcurrently(
					func() { outSlice0, errSlice0 = toSliceAndErrors(outT) },
					func() { outSlice1, errSlice1 = toSliceAndErrors(outF) },
				)

				if len(errSlice0) == 0 || len(errSlice1) == 0 {
					t.Errorf("expected at least one error in each channel")
				}

				allErrsSlice = append(allErrsSlice, errSlice0...)
				allErrsSlice = append(allErrsSlice, errSlice1...)

				th.Sort(outSlice0)
				th.Sort(outSlice1)
				th.Sort(allErrsSlice)

				th.ExpectSlice(t, outSlice0, expectedSlice0)
				th.ExpectSlice(t, outSlice1, expectedSlice1)
				th.ExpectSlice(t, allErrsSlice, expectedAllErrsSLice)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20000), nil)

				outT, outF := universalSplit2(ord, in, n, func(x int) (int, error) {
					if x%3 == 2 {
						return 0, fmt.Errorf("err%06d", x)
					}
					return x % 3, nil
				})

				var outSlice0, outSlice1 []int
				var errSlice0, errSlice1 []string

				th.DoConcurrently(
					func() { outSlice0, errSlice0 = toSliceAndErrors(outT) },
					func() { outSlice1, errSlice1 = toSliceAndErrors(outF) },
				)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice0)
					th.ExpectSorted(t, errSlice0)
					th.ExpectSorted(t, outSlice1)
					th.ExpectSorted(t, errSlice1)
				} else {
					th.ExpectUnsorted(t, outSlice0)
					th.ExpectUnsorted(t, errSlice0)
					th.ExpectUnsorted(t, outSlice1)
					th.ExpectUnsorted(t, errSlice1)
				}
			})

		}
	})
}

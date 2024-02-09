package echans

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func universalSplit2[A any](ord bool, in <-chan Try[A], n int, f func(A) (bool, error)) (<-chan Try[A], <-chan Try[A]) {
	if ord {
		return OrderedSplit2(in, n, f)
	}
	return Split2(in, n, f)
}

func TestSplit2(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				outT, outF := universalSplit2(ord, nil, 5, func(string) (bool, error) { return true, nil })
				th.ExpectValue(t, outT, nil)
				th.ExpectValue(t, outF, nil)

			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 100), nil)
				in = OrderedMap(in, 5, func(x int) (int, error) {
					if x%3 == 2 {
						return 0, fmt.Errorf("err%03d", x)
					}
					return x, nil
				})

				outT, outF := universalSplit2(ord, in, n, func(x int) (bool, error) {
					switch x % 3 {
					case 2:
						return false, fmt.Errorf("err%03d", x)
					case 1:
						return false, nil
					default:
						return true, nil
					}
				})

				expectedSliceT := make([]int, 0, 100)
				expectedSliceF := make([]int, 0, 100)
				expectedAllErrsSLice := make([]string, 0, 50)

				for i := 0; i < 100; i++ {
					switch i % 3 {
					case 2:
						expectedAllErrsSLice = append(expectedAllErrsSLice, fmt.Sprintf("err%03d", i))
					case 1:
						expectedSliceF = append(expectedSliceF, i)
					default:
						expectedSliceT = append(expectedSliceT, i)
					}
				}

				var outSliceT, outSliceF []int
				var errSliceT, errSliceF, allErrsSlice []string

				th.DoConcurrently(
					func() { outSliceT, errSliceT = toSliceAndErrors(outT) },
					func() { outSliceF, errSliceF = toSliceAndErrors(outF) },
				)

				if len(errSliceT) == 0 || len(errSliceF) == 0 {
					t.Errorf("expected at least one error in each channel")
				}

				allErrsSlice = append(allErrsSlice, errSliceT...)
				allErrsSlice = append(allErrsSlice, errSliceF...)

				th.Sort(outSliceT)
				th.Sort(outSliceF)
				th.Sort(allErrsSlice)

				th.ExpectSlice(t, outSliceT, expectedSliceT)
				th.ExpectSlice(t, outSliceF, expectedSliceF)
				th.ExpectSlice(t, allErrsSlice, expectedAllErrsSLice)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := Wrap(th.FromRange(0, 20000), nil)

				outT, outF := universalSplit2(ord, in, n, func(x int) (bool, error) {
					switch x % 3 {
					case 2:
						return false, fmt.Errorf("err%06d", x)
					case 1:
						return false, nil
					default:
						return true, nil
					}
				})

				var outSliceT, outSliceF []int
				var errSliceT, errSliceF []string

				th.DoConcurrently(
					func() { outSliceT, errSliceT = toSliceAndErrors(outT) },
					func() { outSliceF, errSliceF = toSliceAndErrors(outF) },
				)

				if ord || n == 1 {
					th.ExpectSorted(t, outSliceT)
					th.ExpectSorted(t, errSliceT)
					th.ExpectSorted(t, outSliceF)
					th.ExpectSorted(t, errSliceF)
				} else {
					th.ExpectUnsorted(t, outSliceT)
					th.ExpectUnsorted(t, errSliceT)
					th.ExpectUnsorted(t, outSliceF)
					th.ExpectUnsorted(t, errSliceF)
				}

			})

		}
	})
}

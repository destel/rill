package echans

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func doSplit2[A any](ord bool, in <-chan Try[A], n int, f func(A) (bool, error)) (<-chan Try[A], <-chan Try[A]) {
	if ord {
		return OrderedSplit2(in, n, f)
	}
	return Split2(in, n, f)
}

func TestSplit2(t *testing.T) {
	for _, ord := range []bool{false, true} {
		t.Run(testname("nil", ord), func(t *testing.T) {
			outT, outF := doSplit2(ord, nil, 5, func(string) (bool, error) { return true, nil })
			th.ExpectValue(t, outT, nil)
			th.ExpectValue(t, outF, nil)

		})

		t.Run(testname("correctness", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 50), nil)
			in = OrderedMap(in, 5, func(x int) (int, error) {
				if x%3 == 1 {
					return 0, fmt.Errorf("err%03d", x)
				}
				return x, nil
			})

			outT, outF := doSplit2(ord, in, 5, func(x int) (bool, error) {
				if x%3 == 2 {
					return false, fmt.Errorf("err%03d", x)
				}

				return x%2 == 0, nil
			})

			expectedSliceT := make([]int, 0, 50)
			expectedSliceF := make([]int, 0, 50)
			expectedAllErrsSLice := make([]string, 0, 50)

			for i := 0; i < 50; i++ {
				if i%3 != 0 {
					expectedAllErrsSLice = append(expectedAllErrsSLice, fmt.Sprintf("err%03d", i))
					continue
				}
				if i%2 == 0 {
					expectedSliceT = append(expectedSliceT, i)
				} else {
					expectedSliceF = append(expectedSliceF, i)
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

		t.Run(testname("ordering", ord), func(t *testing.T) {
			in := Wrap(th.FromRange(0, 1000), nil)

			outT, outF := doSplit2(ord, in, 50, func(x int) (bool, error) {
				if x%3 == 0 {
					return false, fmt.Errorf("err%06d", x)
				}

				return x%2 == 0, nil
			})

			var outSliceT, outSliceF []int
			var errSliceT, errSliceF []string

			th.DoConcurrently(
				func() { outSliceT, errSliceT = toSliceAndErrors(outT) },
				func() { outSliceF, errSliceF = toSliceAndErrors(outF) },
			)

			if ord {
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

}

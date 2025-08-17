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

func universalSplit2[A any](ord bool, in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	if ord {
		return OrderedSplit2(in, n, f)
	}
	return Split2(in, n, f)
}

func TestSplit2(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		for _, n := range []int{1, 5} {
			t.Run(th.Name("nil", n), func(t *testing.T) {
				outTrue, outFalse := universalSplit2(ord, nil, n, func(string) (bool, error) { return true, nil })
				th.ExpectValue(t, outTrue, nil)
				th.ExpectValue(t, outFalse, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				// idea: split input into 4 groups
				// - first 2 groups are sent into corresponding outputs
				// - 3rd would cause error during splitting
				// - 4th would be errors even before splitting

				in := FromChan(th.FromRange(0, 20*4), nil)
				in = OrderedMap(in, 1, func(x int) (int, error) {
					if x%4 == 3 {
						return 0, fmt.Errorf("err%03d", x)
					}
					return x, nil
				})

				outTrue, outFalse := universalSplit2(ord, in, n, func(x int) (bool, error) {
					switch x % 4 {
					case 0:
						return true, nil
					case 1:
						return false, nil
					case 2:
						return true, fmt.Errorf("err%03d", x)
					default:
						return true, nil // this should not be called
					}
				})

				var outSliceTrue, outSliceFalse []int
				var errSliceTrue, errSliceFalse []string

				th.DoConcurrently(
					func() { outSliceTrue, errSliceTrue = toSliceAndErrors(outTrue) },
					func() { outSliceFalse, errSliceFalse = toSliceAndErrors(outFalse) },
				)

				var expectedOutSliceTrue, expectedOutSliceFalse []int
				var expectedErrSlice []string

				for i := 0; i < 20*4; i++ {
					switch i % 4 {
					case 0:
						expectedOutSliceTrue = append(expectedOutSliceTrue, i)
					case 1:
						expectedOutSliceFalse = append(expectedOutSliceFalse, i)
					default:
						expectedErrSlice = append(expectedErrSlice, fmt.Sprintf("err%03d", i))
					}
				}

				th.Sort(outSliceTrue)
				th.Sort(outSliceFalse)
				th.Sort(errSliceTrue)
				th.Sort(errSliceFalse)

				th.ExpectSlice(t, outSliceTrue, expectedOutSliceTrue)
				th.ExpectSlice(t, outSliceFalse, expectedOutSliceFalse)
				th.ExpectSlice(t, errSliceTrue, expectedErrSlice)
				th.ExpectSlice(t, errSliceFalse, expectedErrSlice)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 10000*4), nil)

				outTrue, outFalse := universalSplit2(ord, in, n, func(x int) (bool, error) {
					switch x % 3 {
					case 0:
						return true, nil
					case 1:
						return false, nil
					default:
						return true, fmt.Errorf("err%06d", x)
					}
				})

				var outSliceTrue, outSliceFalse []int
				var errSliceTrue, errSliceFalse []string

				th.DoConcurrently(
					func() { outSliceTrue, errSliceTrue = toSliceAndErrors(outTrue) },
					func() { outSliceFalse, errSliceFalse = toSliceAndErrors(outFalse) },
				)

				if ord || n == 1 {
					th.ExpectSorted(t, outSliceTrue)
					th.ExpectSorted(t, outSliceFalse)
					th.ExpectSorted(t, errSliceTrue)
					th.ExpectSorted(t, errSliceFalse)
				} else {
					th.ExpectUnsorted(t, outSliceTrue)
					th.ExpectUnsorted(t, outSliceFalse)
					th.ExpectUnsorted(t, errSliceTrue)
					th.ExpectUnsorted(t, errSliceFalse)
				}
			})

		}
	})
}

func TestTee(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		out1, out2 := Tee[int](nil)
		th.ExpectValue(t, out1, nil)
		th.ExpectValue(t, out2, nil)
	})

	t.Run("correctness", func(t *testing.T) {
		// Create input with mixed values and errors
		in := FromChan(th.FromRange(0, 10), nil)
		in = replaceWithError(in, 2, fmt.Errorf("err2"))
		in = replaceWithError(in, 7, fmt.Errorf("err7"))

		out1, out2 := Tee(in)

		var out1Slice, out2Slice []int
		var out1Err, out2Err []string

		th.DoConcurrently(
			func() { out1Slice, out1Err = toSliceAndErrors(out1) },
			func() { out2Slice, out2Err = toSliceAndErrors(out2) },
		)

		expected := []int{0, 1, 3, 4, 5, 6, 8, 9}
		expectedErr := []string{"err2", "err7"}

		// Both outputs should be identical
		th.ExpectSlice(t, out1Slice, expected)
		th.ExpectSlice(t, out2Slice, expected)
		th.ExpectSlice(t, out1Err, expectedErr)
		th.ExpectSlice(t, out2Err, expectedErr)
	})
}

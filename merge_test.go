package rill

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

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

			th.RunSynctest(t, th.Name("correctness", n), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 20), nil)
				in = replaceWithError(in, 15, fmt.Errorf("err015")) // error before splitting

				outTrue, outFalse := universalSplit2(ord, in, n, func(x int) (bool, error) {
					th.SimulateWork(1*time.Second, 2*time.Second)
					if x == 5 || x == 6 {
						return x == 5, fmt.Errorf("err%03d", x) // error during splitting; bool must be ignored
					}
					return x%2 == 0, nil
				})

				var outSliceTrue, outSliceFalse []Item[int]
				th.DoConcurrently(
					func() { outSliceTrue = toItemSlice(outTrue) },
					func() { outSliceFalse = toItemSlice(outFalse) },
				)

				var expectedTrue, expectedFalse []Item[int]
				for i := range 20 {
					switch {
					case i == 5 || i == 6 || i == 15:
						// errors are broadcast to BOTH outputs
						expectedTrue = appendErr(expectedTrue, fmt.Errorf("err%03d", i))
						expectedFalse = appendErr(expectedFalse, fmt.Errorf("err%03d", i))
					case i%2 == 0:
						expectedTrue = appendVal(expectedTrue, i)
					default:
						expectedFalse = appendVal(expectedFalse, i)
					}
				}

				th.ExpectElementsMatch(t, outSliceTrue, expectedTrue)
				th.ExpectElementsMatch(t, outSliceFalse, expectedFalse)
			})

			t.Run(th.Name("non concurrent reads", n), func(t *testing.T) {
				th.ExpectBlock(t, func(t *testing.T) {
					in := FromChan(th.FromRange(0, 20), nil)
					out1, out2 := universalSplit2(ord, in, n, func(x int) (bool, error) {
						return x%2 == 0, nil
					})

					// Reading out1 blocks forever: the producer gets stuck sending to the unread out2,
					// so it stops feeding out1 too. The second call is never reached.
					toItemSlice(out1)
					toItemSlice(out2)
				})
			})

			th.RunSynctest(t, th.Name("ordering", n), func(t *testing.T) {
				in := FromChan(th.FromRange(0, 1000), nil)

				outTrue, outFalse := universalSplit2(ord, in, n, func(x int) (bool, error) {
					th.SimulateWork(1*time.Second, 2*time.Second)
					if x%7 == 0 {
						time.Sleep(10 * time.Second) // force out-of-order completion
					}

					switch x % 3 {
					case 0:
						return true, nil
					case 1:
						return false, nil
					default:
						return true, fmt.Errorf("%03d-err", x)
					}
				})

				var outSliceTrue, outSliceFalse []string

				th.DoConcurrently(
					func() { outSliceTrue = toUnifiedStringSlice(outTrue, "%03d") },
					func() { outSliceFalse = toUnifiedStringSlice(outFalse, "%03d") },
				)

				if ord || n == 1 {
					th.ExpectSorted(t, outSliceTrue)
					th.ExpectSorted(t, outSliceFalse)
				} else {
					th.ExpectUnsorted(t, outSliceTrue)
					th.ExpectUnsorted(t, outSliceFalse)
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

	th.RunSynctest(t, th.Name("correctness"), func(t *testing.T) {
		// Create input with mixed values and errors
		in := FromChan(th.FromRange(0, 10), nil)
		in = replaceWithError(in, 2, fmt.Errorf("err2"))
		in = replaceWithError(in, 7, fmt.Errorf("err7"))

		out1, out2 := Tee(in)

		var outSlice1, outSlice2 []Item[int]

		th.DoConcurrently(
			func() { outSlice1 = toItemSlice(out1) },
			func() { outSlice2 = toItemSlice(out2) },
		)

		var expected []Item[int]
		expected = appendVal(expected, 0, 1)
		expected = appendErr(expected, fmt.Errorf("err2"))
		expected = appendVal(expected, 3, 4, 5, 6)
		expected = appendErr(expected, fmt.Errorf("err7"))
		expected = appendVal(expected, 8, 9)

		// Both outputs should be identical
		th.ExpectSlice(t, outSlice1, expected)
		th.ExpectSlice(t, outSlice2, expected)
	})

	t.Run(th.Name("non concurrent reads"), func(t *testing.T) {
		th.ExpectBlock(t, func(t *testing.T) {
			in := FromChan(th.FromRange(0, 10), nil)
			out1, out2 := Tee(in)

			// Reading out1 blocks forever: the producer gets stuck sending to the unread out2,
			// so it stops feeding out1 too. The second call is never reached.
			toItemSlice(out1)
			toItemSlice(out2)
		})
	})
}

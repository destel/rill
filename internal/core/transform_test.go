package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func universalFilterMap[A, B any](ord bool, in <-chan A, n int, f func(A) (B, bool)) <-chan B {
	if ord {
		return OrderedFilterMap(in, n, f)
	}
	return FilterMap(in, n, f)
}

func TestFilterMap(t *testing.T) {
	th.TestBothOrderings(t, func(t *testing.T, ord bool) {
		th.TestLevels(t, []int{1, 5}, func(t *testing.T, n int) {

			t.Run("nil", func(t *testing.T) {
				out := universalFilterMap(ord, nil, n, func(x int) (int, bool) { return x, true })
				th.ExpectValue(t, out, nil)
			})

			th.RunSynctest(t, "correctness", func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := universalFilterMap(ord, in, n, func(x int) (string, bool) {
					return fmt.Sprintf("%03d", x), x%2 == 0
				})

				outSlice := th.ToSlice(out)

				var expectedSlice []string
				for i := range 20 {
					if i%2 != 0 {
						continue
					}
					expectedSlice = append(expectedSlice, fmt.Sprintf("%03d", i))
				}

				th.ExpectElementsMatch(t, outSlice, expectedSlice)
			})

			th.RunSynctest(t, "ordering", func(t *testing.T) {
				in := th.FromRange(0, 100)

				out := universalFilterMap(ord, in, n, func(x int) (int, bool) {
					if x%7 == 0 {
						time.Sleep(1 * time.Second) // force out-of-order completion
					}
					return x, x%2 == 0
				})

				outSlice := th.ToSlice(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		})
	})
}

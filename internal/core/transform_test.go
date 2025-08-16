package core

import (
	"fmt"
	"testing"

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
		for _, n := range []int{1, 5} {

			t.Run(th.Name("nil", n), func(t *testing.T) {
				out := universalFilterMap(ord, nil, n, func(x int) (int, bool) { return x, true })
				th.ExpectValue(t, out, nil)
			})

			t.Run(th.Name("correctness", n), func(t *testing.T) {
				in := th.FromRange(0, 20)
				out := universalFilterMap(ord, in, n, func(x int) (string, bool) {
					return fmt.Sprintf("%03d", x), x%2 == 0
				})

				outSlice := th.ToSlice(out)

				expectedSlice := make([]string, 0, 20)
				for i := 0; i < 20; i++ {
					if i%2 != 0 {
						continue
					}
					expectedSlice = append(expectedSlice, fmt.Sprintf("%03d", i))
				}

				th.Sort(outSlice)
				th.ExpectSlice(t, outSlice, expectedSlice)
			})

			t.Run(th.Name("ordering", n), func(t *testing.T) {
				in := th.FromRange(0, 20000)

				out := universalFilterMap(ord, in, n, func(x int) (int, bool) {
					return x, x%2 == 0
				})

				outSlice := th.ToSlice(out)

				if ord || n == 1 {
					th.ExpectSorted(t, outSlice)
				} else {
					th.ExpectUnsorted(t, outSlice)
				}
			})

		}
	})
}

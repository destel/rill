package echans

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestToFromSlice(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		s := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		in := FromSlice(s)

		s1, err := ToSlice(in)
		th.ExpectNoError(t, err)
		th.ExpectSlice(t, s1, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	})

	t.Run("errors", func(t *testing.T) {
		s := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		in := FromSlice(s)

		in = replaceWithError(in, 4, fmt.Errorf("err1"))
		in = replaceWithError(in, 8, fmt.Errorf("err2"))

		s1, err := ToSlice(in)
		th.ExpectError(t, err, "err1")
		th.ExpectSlice(t, s1, []int{0, 1, 2, 3})
	})
}

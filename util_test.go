package rill

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestDrain(t *testing.T) {
	// real tests are in another package
	Drain[int](th.FromRange(0, 10))
	DrainNB[int](th.FromRange(0, 10))
}

func TestBuffer(t *testing.T) {
	// real tests are in another package
	Buffer[int](th.FromRange(0, 10), 5)
}

func TestFromToSlice(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		inSlice := make([]int, 20)
		for i := 0; i < 20; i++ {
			inSlice[i] = i
		}

		in := FromSlice(inSlice)
		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, inSlice)
		th.ExpectNoError(t, err)
	})

	t.Run("errors", func(t *testing.T) {
		inSlice := make([]int, 20)
		for i := 0; i < 20; i++ {
			inSlice[i] = i
		}

		in := FromSlice(inSlice)
		in = replaceWithError(in, 15, fmt.Errorf("err15"))
		in = replaceWithError(in, 18, fmt.Errorf("err18"))

		outSlice, err := ToSlice(in)

		th.ExpectSlice(t, outSlice, inSlice[:15])
		th.ExpectError(t, err, "err15")

		time.Sleep(1 * time.Second)
		th.ExpectDrainedChan(t, in)
	})
}

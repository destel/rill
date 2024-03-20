package common

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
			outSlice := th.ToSlice(out)

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

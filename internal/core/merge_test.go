package core

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestMerge(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := Merge[string]()
		th.ExpectValue(t, out, nil)
	})

	makeSubChan := func(chanID int) <-chan string {
		ch := make(chan string)
		go func() {
			defer close(ch)
			ch <- fmt.Sprintf("%03dA", chanID)
			ch <- fmt.Sprintf("%03dB", chanID)
			ch <- fmt.Sprintf("%03dC", chanID)
			ch <- fmt.Sprintf("%03dD", chanID)
		}()
		return ch
	}

	appendExpected := func(acc []string, chanID int) []string {
		return append(acc,
			fmt.Sprintf("%03dA", chanID),
			fmt.Sprintf("%03dB", chanID),
			fmt.Sprintf("%03dC", chanID),
			fmt.Sprintf("%03dD", chanID),
		)
	}

	for _, numChans := range []int{1, 3, 5, 10} {
		th.RunSynctest(t, th.Name("correctness", numChans), func(t *testing.T) {
			ins := make([]<-chan string, numChans)

			for i := range numChans {
				ins[i] = makeSubChan(i)
			}

			out := Merge(ins...)
			outSlice := th.ToSlice(out)

			var expectedSlice []string
			for i := range numChans {
				expectedSlice = appendExpected(expectedSlice, i)
			}

			th.ExpectElementsMatch(t, outSlice, expectedSlice)
		})

		t.Run(th.Name("nil hang", numChans), func(t *testing.T) {
			var outSlice []string

			th.ExpectBlock(t, func(t *testing.T) {
				ins := make([]<-chan string, numChans)
				for i := 0; i < numChans-1; i++ {
					ins[i] = makeSubChan(i)
				}

				out := Merge(ins...)
				for x := range out {
					outSlice = append(outSlice, x)
				}
			})

			var expectedSlice []string
			for i := 0; i < numChans-1; i++ {
				expectedSlice = appendExpected(expectedSlice, i)
			}

			th.ExpectElementsMatch(t, outSlice, expectedSlice)
		})
	}
}

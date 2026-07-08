package core

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
		th.RunSynctest(t, th.Name("correctness", numChans), func(t *testing.T) {
			ins := make([]<-chan int, numChans)

			for i := range numChans {
				ins[i] = th.FromRange(20*i, 20*(i+1))
			}

			out := Merge(ins...)
			outSlice := th.ToSlice(out)

			var expectedSlice []int
			for i := range 20 * numChans {
				expectedSlice = append(expectedSlice, i)
			}

			th.ExpectElementsMatch(t, outSlice, expectedSlice)
		})

		th.RunSynctest(t, th.Name("per input independence", numChans), func(t *testing.T) {
			ins := make([]<-chan int, numChans)
			insWritable := make([]chan int, numChans)
			for i := range numChans {
				ch := make(chan int)
				insWritable[i] = ch
				ins[i] = ch
			}

			go func() {
				// write to the channels in a round-robin fashion
				for i := 0; i < 100; i++ {
					th.SimulateWork(1*time.Second, 2*time.Second)
					insWritable[i%len(insWritable)] <- i
				}

				for _, ch := range insWritable {
					close(ch)
				}
			}()

			out := Merge(ins...)
			outSlice := th.ToSlice(out)

			var expectedSlice []int
			for i := 0; i < 100; i++ {
				expectedSlice = append(expectedSlice, i)
			}

			th.ExpectSlice(t, outSlice, expectedSlice) // exact order
		})

		t.Run(th.Name("nil hang", numChans), func(t *testing.T) {
			th.ExpectBlock(t, func(t *testing.T) {
				ins := make([]<-chan int, numChans)
				for i := range numChans {
					if i == len(ins)/2 {
						continue // leave the middle channel nil
					}
					ins[i] = th.FromRange(20*i, 20*(i+1))
				}

				out := Merge(ins...)
				Drain(out)
			})
		})
	}
}

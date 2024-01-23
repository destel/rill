package chans

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestMerge(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		th.ExpectValue(t, Merge[string](), nil)
	})

	testCorrectness := func(t *testing.T, cnt int) {
		t.Run(fmt.Sprintf("correctness %d", cnt), func(t *testing.T) {
			ins := make([]<-chan int, cnt)

			// each channel has 10 elements
			for i := 0; i < cnt; i++ {
				ins[i] = th.FromRange(i*10, (i+1)*10)
			}

			if cnt > 0 {
				ins[0] = Map(ins[0], 1, func(x int) int {
					// break the ordering: make 8th element of the first channel slow
					if x == 8 {
						time.Sleep(1 * time.Second)
					}
					return x
				})
			}

			out := Merge(ins...)
			outSlice := ToSlice(out)

			if cnt > 1 && sort.IntsAreSorted(outSlice) {
				t.Errorf("expected outSlice to be unsorted")
			}

			var expected []int
			for i := 0; i < cnt*10; i++ {
				expected = append(expected, i)
			}

			sort.Ints(outSlice)
			th.ExpectSlice(t, outSlice, expected)
		})

	}

	testNilHang := func(t *testing.T, cnt int) {
		t.Run(fmt.Sprintf("nil hang %d", cnt), func(t *testing.T) {
			ins := make([]<-chan int, cnt)

			// cnt-1 normal channels
			for i := 0; i < cnt-1; i++ {
				ins[i] = th.FromRange(i*10, (i+1)*10)
			}

			// single nil channel that should make the whole thing to hang
			ins[cnt-1] = nil

			out := Merge(ins...)

			// read as much as we can in 1 second
			var outSlice []int
			timer := time.After(1 * time.Second)
		Loop:
			for {
				select {
				case x, ok := <-out:
					if !ok {
						t.Errorf("hang expected, but channel was closed")
					}
					outSlice = append(outSlice, x)

				case <-timer:
					break Loop
				}

			}

			var expected []int
			for i := 0; i < (cnt-1)*10; i++ {
				expected = append(expected, i)
			}

			sort.Ints(outSlice)
			th.ExpectSlice(t, outSlice, expected)
		})
	}

	testCorrectness(t, 1)
	testCorrectness(t, 3)
	testCorrectness(t, 5)
	testCorrectness(t, 10)

	testNilHang(t, 1)
	testNilHang(t, 3)
	testNilHang(t, 5)
	testNilHang(t, 10)
}

func TestSplit2(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		outT, outF := Split2(nil, 3, func(x int) bool { return true })
		th.ExpectValue(t, outT, nil)
		th.ExpectValue(t, outF, nil)
	})

	t.Run("correctness", func(t *testing.T) {
		in := th.FromRange(0, 20)
		outT, outF := Split2(in, 3, func(x int) bool {
			return x%2 == 0
		})

		// Buffer the channels to avoid deadlocks
		// Without it, we'd have to call ToSlice(outT) and ToSlice(outF) concurrently
		outT = Buffer(outT, 20)
		outF = Buffer(outF, 20)

		outTslice := ToSlice(outT)
		outFslice := ToSlice(outF)

		sort.Ints(outTslice)
		sort.Ints(outFslice)

		th.ExpectSlice(t, outTslice, []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18})
		th.ExpectSlice(t, outFslice, []int{1, 3, 5, 7, 9, 11, 13, 15, 17, 19})
	})

	t.Run("concurrency", func(t *testing.T) {
		var inProgress th.InProgressCounter

		in := th.FromRange(0, 20)
		outT, outF := Split2(in, 10, func(x int) bool {
			inProgress.Inc()
			defer inProgress.Dec()

			time.Sleep(1 * time.Second)
			return x%2 == 0
		})

		Drain(Merge(outT, outF))

		th.ExpectValue(t, inProgress.Max(), 10)
	})

}

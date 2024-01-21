package chans

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

func TestMerge(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		expectValue(t, nil, Merge[string]())
	})

	testCorrectness := func(t *testing.T, cnt int) {
		t.Run(fmt.Sprintf("correctness %d", cnt), func(t *testing.T) {
			ins := make([]<-chan int, cnt)

			for i := 0; i < cnt; i++ {
				ins[i] = fromRange(i*10, (i+1)*10)
			}

			if cnt > 0 {
				ins[0] = Map(ins[0], 1, func(x int) int {
					// break the ordering
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
			expectSlice(t, expected, outSlice)
		})

	}

	testNilHang := func(t *testing.T, cnt int) {
		t.Run(fmt.Sprintf("nil hang %d", cnt), func(t *testing.T) {
			ins := make([]<-chan int, cnt)

			// cnt-1 normal channels
			for i := 0; i < cnt-1; i++ {
				ins[i] = fromRange(i*10, (i+1)*10)
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
			expectSlice(t, expected, outSlice)
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
		outT, outF := Split2(nil, func(x int) bool { return true })
		expectValue(t, nil, outT)
		expectValue(t, nil, outF)
	})

	t.Run("correctness", func(t *testing.T) {
		in := fromRange(0, 20)
		outT, outF := Split2(in, func(x int) bool {
			return x%2 == 0
		})

		// Buffer the channels to avoid deadlocks
		// Without is we can't call ToSlice(outT) and ToSlice(outF) sequentially
		outT = Buffer(outT, 20)
		outF = Buffer(outF, 20)

		outTslice := ToSlice(outT)
		outFslice := ToSlice(outF)

		var expectedT []int
		var expectedF []int

		for i := 0; i < 20; i++ {
			if i%2 == 0 {
				expectedT = append(expectedT, i)
			} else {
				expectedF = append(expectedF, i)
			}
		}

		expectSlice(t, expectedT, outTslice)
		expectSlice(t, expectedF, outFslice)
	})

}

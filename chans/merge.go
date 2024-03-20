package chans

import (
	"sync"

	"github.com/destel/rill/internal/common"
)

func fastMerge[A any](ins []<-chan A) <-chan A {
	// len(ins) must be between 2 and 5

	remaining := len(ins)
	for len(ins) < 5 {
		ins = append(ins, nil)
	}

	out := make(chan A)

	go func() {
		defer close(out)

		var a A
		var ok bool
		var i int

		for {
			if remaining == 0 {
				return
			}

			select {
			case a, ok = <-ins[0]:
				i = 0
			case a, ok = <-ins[1]:
				i = 1
			case a, ok = <-ins[2]:
				i = 2
			case a, ok = <-ins[3]:
				i = 3
			case a, ok = <-ins[4]:
				i = 4
			}

			if !ok {
				remaining--
				ins[i] = nil
				continue
			}

			out <- a
		}
	}()

	return out
}

func slowMerge[A any](ins []<-chan A) <-chan A {
	out := make(chan A)

	var wg sync.WaitGroup
	for _, in := range ins {
		in1 := in
		wg.Add(1)
		go func() {
			defer wg.Done()
			for x := range in1 {
				out <- x
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// Merge combines multiple input channels into a single output channel. Items are emitted as soon as they're available,
// so the output order is not defined.
func Merge[A any](ins ...<-chan A) <-chan A {
	switch len(ins) {
	case 0:
		return nil
	case 1:
		return ins[0]
	case 2, 3, 4, 5:
		return fastMerge(ins)
	default:
		return slowMerge(ins)
	}
}

// Split2 divides the input channel into two output channels based on the discriminator function f, using n goroutines for concurrency.
// The function f takes an item from the input and decides which output channel (out0 or out1) it should go to by returning 0 or 1, respectively.
// Return values other than 0 or 1 lead to the item being discarded.
// The output order is not guaranteed: results are written to the outputs as soon as they're ready.
// Use OrderedSplit2 to preserve the input order.
func Split2[A any](in <-chan A, n int, f func(A) int) (out0 <-chan A, out1 <-chan A) {
	outs := common.MapAndSplit(in, 2, n, func(a A) (A, int) {
		return a, f(a)
	})
	return outs[0], outs[1]
}

// OrderedSplit2 is similar to Split2, but it guarantees that the order of the outputs matches the order of the input.
func OrderedSplit2[A any](in <-chan A, n int, f func(A) int) (out0 <-chan A, out1 <-chan A) {
	outs := common.OrderedMapAndSplit(in, 2, n, func(a A) (A, int) {
		return a, f(a)
	})
	return outs[0], outs[1]
}

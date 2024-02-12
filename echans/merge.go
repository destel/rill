package echans

import (
	"math/rand"

	"github.com/destel/rill/chans"
	"github.com/destel/rill/internal/common"
)

func Merge[A any](ins ...<-chan A) <-chan A {
	return chans.Merge(ins...)
}

func Split2[A any](in <-chan Try[A], n int, f func(A) (int, error)) (out0 <-chan Try[A], out1 <-chan Try[A]) {
	outs := common.MapAndSplit(in, 2, n, func(a Try[A]) (Try[A], int) {
		if a.Error != nil {
			return a, rand.Int() & 1
		}

		i, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, rand.Int() & 1
		}

		return a, i
	})

	return outs[0], outs[1]
}

func OrderedSplit2[A any](in <-chan Try[A], n int, f func(A) (int, error)) (out0 <-chan Try[A], out1 <-chan Try[A]) {
	outs := common.OrderedMapAndSplit(in, 2, n, func(a Try[A]) (Try[A], int) {
		if a.Error != nil {
			return a, rand.Int() & 1
		}

		i, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, rand.Int() & 1
		}

		return a, i
	})

	return outs[0], outs[1]
}

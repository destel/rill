package echans

import (
	"math/rand"

	"github.com/destel/rill/chans"
	"github.com/destel/rill/internal/common"
)

func Merge[A any](ins ...<-chan A) <-chan A {
	return chans.Merge(ins...)
}

func Split2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	outs := common.MapAndSplit(in, n, 2, func(a Try[A]) (Try[A], int) {
		if a.Error != nil {
			return a, rand.Int() & 1
		}

		side, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, rand.Int() & 1
		}

		if side {
			return a, 0
		} else {
			return a, 1
		}
	})

	return outs[0], outs[1]
}

func OrderedSplit2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	res := common.OrderedMapAndSplit(in, n, 2, func(a Try[A]) (Try[A], int) {
		if a.Error != nil {
			return a, rand.Int() & 1
		}

		side, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, rand.Int() & 1
		}

		if side {
			return a, 0
		} else {
			return a, 1
		}
	})

	return res[0], res[1]
}

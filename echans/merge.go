package echans

import (
	"math/rand"

	"github.com/destel/rill/chans"
)

func Merge[A any](ins ...<-chan A) <-chan A {
	return chans.Merge(ins...)
}

func Split2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	tmp := chans.Map(in, n, func(a Try[A]) tuple2[Try[A], bool] {
		if a.Error != nil {
			return makeTuple2(a, rand.Int63()&1 == 1)
		}

		side, err := f(a.V)
		if err != nil {
			return makeTuple2(Try[A]{Error: err}, rand.Int63()&1 == 1)
		}

		return makeTuple2(a, side)
	})

	tmpTrue, tmpFalse := chans.Split2(tmp, 1, func(a tuple2[Try[A], bool]) bool {
		return a.V2
	})

	return tuple2chanExtract1(tmpTrue), tuple2chanExtract1(tmpFalse)
}

func OrderedSplit2[A any](in <-chan Try[A], n int, f func(A) (bool, error)) (outTrue <-chan Try[A], outFalse <-chan Try[A]) {
	tmp := chans.OrderedMap(in, n, func(a Try[A]) tuple2[Try[A], bool] {
		if a.Error != nil {
			return makeTuple2(a, rand.Int63()&1 == 1)
		}

		side, err := f(a.V)
		if err != nil {
			return makeTuple2(Try[A]{Error: err}, rand.Int63()&1 == 1)
		}

		return makeTuple2(a, side)
	})

	tmpTrue, tmpFalse := chans.Split2(tmp, 1, func(a tuple2[Try[A], bool]) bool {
		return a.V2
	})

	return tuple2chanExtract1(tmpTrue), tuple2chanExtract1(tmpFalse)
}

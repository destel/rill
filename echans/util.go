package echans

import (
	"github.com/destel/rill/chans"
)

func Drain[A any](in <-chan A) {
	chans.Drain(in)
}

func DrainNB[A any](in <-chan A) {
	chans.DrainNB(in)
}

func Buffer[A any](in <-chan A, n int) <-chan A {
	return chans.Buffer(in, n)
}

func FromSlice[A any](slice []A) <-chan Try[A] {
	out := make(chan Try[A])
	go func() {
		defer close(out)
		for _, a := range slice {
			out <- Try[A]{V: a}
		}
	}()
	return out
}

// todo: do early exit or no? what are use cases?
func ToSlice[A any](in <-chan Try[A]) ([]A, error) {
	var res []A

	defer chans.DrainNB(in)

	for x := range in {
		if err := x.Error; err != nil {
			return res, err
		}
		res = append(res, x.V)
	}

	return res, nil
}

type tuple2[A, B any] struct {
	V1 A
	V2 B
}

func makeTuple2[A, B any](v1 A, v2 B) tuple2[A, B] {
	return tuple2[A, B]{V1: v1, V2: v2}
}

// takes chan of tuples and returns chan of first elements of each tuple
func tuple2chanExtract1[A any](in <-chan tuple2[A, bool]) <-chan A {
	return chans.Map(in, 1, func(a tuple2[A, bool]) A {
		return a.V1
	})
}

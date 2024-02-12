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

func ToSlice[A any](in <-chan Try[A]) ([]A, error) {
	var res []A

	for x := range in {
		if err := x.Error; err != nil {
			chans.DrainNB(in)
			return res, err
		}
		res = append(res, x.V)
	}

	return res, nil
}

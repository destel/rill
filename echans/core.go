package echans

import "github.com/destel/rill/chans"

type Try[A any] struct {
	V     A
	Error error
}

func MapAndFilter[A, B any](in <-chan Try[A], n int, f func(A) (B, bool, error)) <-chan Try[B] {
	return chans.MapAndFilter(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, keep, err := f(a.V)
		if err != nil {
			// always keep errors
			return Try[B]{Error: err}, true
		}

		return Try[B]{V: b}, keep
	})
}

func Map[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	return MapAndFilter(in, n, func(a A) (B, bool, error) {
		b, err := f(a)
		return b, true, err
	})
}

func Filter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	return MapAndFilter(in, n, func(a A) (A, bool, error) {
		keep, err := f(a)
		return a, keep, err
	})
}

// todo: think about design of this function
func FlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	return chans.FlatMap(in, n, func(a Try[A]) <-chan Try[B] {
		if a.Error != nil {
			errChan := make(chan Try[B], 1)
			errChan <- Try[B]{Error: a.Error}
			close(errChan)
			return errChan
		}

		return f(a.V)
	})
}
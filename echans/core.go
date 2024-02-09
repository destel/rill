package echans

import (
	"sync"

	"github.com/destel/rill/chans"
)

func MapAndFilter[A, B any](in <-chan Try[A], n int, f func(A) (B, bool, error)) <-chan Try[B] {
	return chans.MapAndFilter(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true // always keep errors
		}

		b, keep, err := f(a.V)
		if err != nil {
			return Try[B]{Error: err}, true // always keep errors
		}

		return Try[B]{V: b}, keep
	})
}

func OrderedMapAndFilter[A, B any](in <-chan Try[A], n int, f func(A) (B, bool, error)) <-chan Try[B] {
	return chans.OrderedMapAndFilter(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true // always keep errors
		}

		b, keep, err := f(a.V)
		if err != nil {
			return Try[B]{Error: err}, true // always keep errors
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

func OrderedMap[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	return OrderedMapAndFilter(in, n, func(a A) (B, bool, error) {
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

func OrderedFilter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	return OrderedMapAndFilter(in, n, func(a A) (A, bool, error) {
		keep, err := f(a)
		return a, keep, err
	})
}

func FlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	return chans.FlatMap(in, n, func(a Try[A]) <-chan Try[B] {
		if a.Error != nil {
			// More optimal implementation base on common.Loop is possible.
			// Here we assume errors are rare.
			errChan := make(chan Try[B], 1)
			errChan <- Try[B]{Error: a.Error}
			close(errChan)
			return errChan
		}

		return f(a.V)
	})
}

func OrderedFlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	return chans.OrderedFlatMap(in, n, func(a Try[A]) <-chan Try[B] {
		if a.Error != nil {
			errChan := make(chan Try[B], 1)
			errChan <- Try[B]{Error: a.Error}
			close(errChan)
			return errChan
		}

		return f(a.V)
	})
}

func Catch[A any](in <-chan Try[A], n int, f func(error) error) <-chan Try[A] {
	return chans.MapAndFilter(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error == nil {
			return a, true
		}

		err := f(a.Error)
		if err == nil {
			return a, false // error handled, filter out
		}

		return Try[A]{Error: err}, true // error replaced by f(a.Error)
	})
}

func OrderedCatch[A any](in <-chan Try[A], n int, f func(error) error) <-chan Try[A] {
	return chans.OrderedMapAndFilter(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error == nil {
			return a, true
		}

		err := f(a.Error)
		if err == nil {
			return a, false // error handled, filter out
		}

		return Try[A]{Error: err}, true // error replaced by f(a.Error)
	})
}

func ForEach[A any](in <-chan Try[A], n int, f func(A) error) error {
	var retErr error
	var once sync.Once

	chans.ForEach(in, n, func(a Try[A]) bool {
		err := a.Error
		if err == nil {
			err = f(a.V)
		}

		if err != nil {
			once.Do(func() {
				retErr = err
			})
			return false // early exit
		}

		return true
	})

	return retErr
}

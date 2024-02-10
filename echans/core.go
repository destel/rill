package echans

import (
	"sync"

	"github.com/destel/rill/chans"
	"github.com/destel/rill/internal/common"
)

func Map[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	return common.MapOrFilter(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, err := f(a.V)
		if err != nil {
			return Try[B]{Error: err}, true
		}

		return Try[B]{V: b}, true
	})
}

func OrderedMap[A, B any](in <-chan Try[A], n int, f func(A) (B, error)) <-chan Try[B] {
	return common.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[B], bool) {
		if a.Error != nil {
			return Try[B]{Error: a.Error}, true
		}

		b, err := f(a.V)
		if err != nil {
			return Try[B]{Error: err}, true
		}

		return Try[B]{V: b}, true
	})
}

func Filter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	return common.MapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error != nil {
			return a, true // never filter out errors
		}

		keep, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, true // never filter out errors
		}

		return a, keep
	})
}

func OrderedFilter[A any](in <-chan Try[A], n int, f func(A) (bool, error)) <-chan Try[A] {
	return common.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
		if a.Error != nil {
			return a, true // never filter out errors
		}

		keep, err := f(a.V)
		if err != nil {
			return Try[A]{Error: err}, true // never filter out errors
		}

		return a, keep
	})
}

func FlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	return common.MapOrFlatMap(in, n, func(a Try[A]) (<-chan Try[B], Try[B]) {
		if a.Error != nil {
			return nil, Try[B]{Error: a.Error}
		}
		return f(a.V), Try[B]{}
	})
}

func OrderedFlatMap[A, B any](in <-chan Try[A], n int, f func(A) <-chan Try[B]) <-chan Try[B] {
	return common.OrderedMapOrFlatMap(in, n, func(a Try[A]) (<-chan Try[B], Try[B]) {
		if a.Error != nil {
			return nil, Try[B]{Error: a.Error}
		}
		return f(a.V), Try[B]{}
	})
}

func Catch[A any](in <-chan Try[A], n int, f func(error) error) <-chan Try[A] {
	return common.MapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
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
	return common.OrderedMapOrFilter(in, n, func(a Try[A]) (Try[A], bool) {
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

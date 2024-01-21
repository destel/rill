package echans

import "github.com/destel/rill/chans"

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
	var err error

	for x := range in {
		if x.Error != nil {
			if err == nil {
				err = x.Error
			}
			continue
		}

		res = append(res, x.V)
	}

	return res, err
}

func Wrap[A any](in <-chan A, err error, asyncErrs <-chan error) <-chan Try[A] {
	toMerge := make([]<-chan Try[A], 0, 3)

	if in != nil {
		toMerge = append(toMerge, chans.Map(in, 1, func(x A) Try[A] {
			return Try[A]{V: x}
		}))
	}

	if err != nil {
		errChan := make(chan Try[A], 1)
		errChan <- Try[A]{Error: err}
		close(errChan)

		toMerge = append(toMerge, errChan)
	}

	if asyncErrs != nil {
		toMerge = append(toMerge, chans.Map(asyncErrs, 1, func(err error) Try[A] {
			return Try[A]{Error: err}
		}))
	}

	return chans.Merge(toMerge...)
}

func Unwrap[A any](in <-chan Try[A]) (<-chan A, <-chan error) {
	if in == nil {
		return nil, nil
	}

	out := make(chan A)
	errs := make(chan error)

	go func() {
		defer close(out)
		defer close(errs)

		for x := range in {
			if x.Error != nil {
				errs <- x.Error
			} else {
				out <- x.V
			}
		}
	}()

	return out, errs
}

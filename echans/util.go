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

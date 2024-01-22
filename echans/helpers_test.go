package echans

func toSliceAndErrors[A any](in <-chan Try[A]) ([]A, []string) {
	var values []A
	var errors []string

	for x := range in {
		if x.Error != nil {
			errors = append(errors, x.Error.Error())
			continue
		}

		values = append(values, x.V)
	}

	return values, errors
}

// putErrorAt replaces item at index with an error
func putErrorAt[A any](in <-chan Try[A], err error, index int) <-chan Try[A] {
	out := make(chan Try[A])

	go func() {
		defer close(out)
		i := -1

		for x := range in {
			i++
			if i == index {
				out <- Try[A]{Error: err}
				continue
			}

			out <- x
		}
	}()

	return out
}

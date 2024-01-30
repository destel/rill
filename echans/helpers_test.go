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

func replaceWithError[A comparable](in <-chan Try[A], value A, err error) <-chan Try[A] {
	out := make(chan Try[A])

	go func() {
		defer close(out)

		for x := range in {
			if x.Error == nil && x.V == value {
				x.Error = err
			}
			out <- x
		}
	}()

	return out
}

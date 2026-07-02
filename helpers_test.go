package rill

import "testing/synctest"

func toSliceAndErrors[A any](in <-chan Try[A]) ([]A, []string) {
	var values []A
	var errors []string

	for x := range in {
		if x.Error != nil {
			errors = append(errors, x.Error.Error())
			continue
		}

		values = append(values, x.Value)
	}

	return values, errors
}

// Similar to toSliceAndErrors, but reads until all goroutines are durably blocked
// The third return is true only if the channel was fully consumed.
func toSliceAndErrorsNB[A any](in <-chan Try[A]) ([]A, []string, bool) {
	var values []A
	var errors []string

	for {
		// Wait for all goroutines to be durably blocked.
		// This makes the non-blocking read below deterministic.
		synctest.Wait()

		select {
		case x, ok := <-in:
			if !ok {
				return values, errors, true
			}

			if x.Error != nil {
				errors = append(errors, x.Error.Error())
				continue
			}

			values = append(values, x.Value)
		default:
			return values, errors, false
		}
	}
}

func replaceWithError[A comparable](in <-chan Try[A], value A, err error) <-chan Try[A] {
	out := make(chan Try[A])

	go func() {
		defer close(out)

		for x := range in {
			if x.Error == nil && x.Value == value {
				x.Error = err
			}
			out <- x
		}
	}()

	return out
}

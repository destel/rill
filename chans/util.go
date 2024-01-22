package chans

func Drain[A any](in <-chan A) {
	for range in {
	}
}

func DrainNB[A any](in <-chan A) {
	for {
		select {
		case _, ok := <-in:
			if !ok {
				return
			}
		default:
			go Drain(in)
			return
		}
	}
}

func Buffer[A any](in <-chan A, n int) <-chan A {
	// we use n+1 since 1 additional is held on the stack (x variable)
	out := make(chan A, n-1)

	go func() {
		defer close(out)
		for x := range in {
			out <- x
		}
	}()

	return out
}

func FromSlice[A any](slice []A) <-chan A {
	out := make(chan A)
	go func() {
		defer close(out)
		for _, a := range slice {
			out <- a
		}
	}()
	return out
}

func ToSlice[A any](in <-chan A) []A {
	var res []A
	for x := range in {
		res = append(res, x)
	}
	return res
}

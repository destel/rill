package common

func fromSlice[A any](slice []A) <-chan A {
	out := make(chan A)
	go func() {
		defer close(out)
		for _, a := range slice {
			out <- a
		}
	}()
	return out
}

func toSlice[A any](in <-chan A) []A {
	var res []A
	for x := range in {
		res = append(res, x)
	}
	return res
}

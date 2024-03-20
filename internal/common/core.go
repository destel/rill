package common

func MapOrFilter[A, B any](in <-chan A, n int, f func(A) (B, bool)) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)

	Loop(in, out, n, func(a A) {
		b, keep := f(a)
		if keep {
			out <- b
		}
	})

	return out
}

func OrderedMapOrFilter[A, B any](in <-chan A, n int, f func(A) (B, bool)) <-chan B {
	if in == nil {
		return nil
	}

	out := make(chan B)
	OrderedLoop(in, out, n, func(a A, canWrite <-chan struct{}) {
		y, keep := f(a)
		<-canWrite
		if keep {
			out <- y
		}
	})

	return out
}

func MapAndSplit[A, B any](in <-chan A, numOuts int, n int, f func(A) (B, int)) []<-chan B {
	if in == nil {
		return make([]<-chan B, numOuts)
	}

	outs := make([]chan B, numOuts)
	outsReadOnly := make([]<-chan B, numOuts)
	for i := range outs {
		out := make(chan B)
		outs[i] = out
		outsReadOnly[i] = out
	}

	done := make(chan struct{})

	Loop(in, done, n, func(a A) {
		b, i := f(a)
		if i >= 0 && i < numOuts {
			outs[i] <- b
		}
	})

	go func() {
		<-done
		for _, out := range outs {
			close(out)
		}
	}()

	return outsReadOnly
}

func OrderedMapAndSplit[A, B any](in <-chan A, numOuts int, n int, f func(A) (B, int)) []<-chan B {
	if in == nil {
		return make([]<-chan B, numOuts)
	}

	outs := make([]chan B, numOuts)
	outsReadOnly := make([]<-chan B, numOuts)
	for i := range outs {
		out := make(chan B)
		outs[i] = out
		outsReadOnly[i] = out
	}

	done := make(chan struct{})

	OrderedLoop(in, done, n, func(a A, canWrite <-chan struct{}) {
		b, i := f(a)
		<-canWrite
		if i >= 0 && i < numOuts {
			outs[i] <- b
		}
	})

	go func() {
		<-done
		for _, out := range outs {
			close(out)
		}
	}()

	return outsReadOnly
}

package core

import (
	"sync"
)

func Merge[A any](ins ...<-chan A) <-chan A {
	switch len(ins) {
	case 0:
		out := make(chan A)
		close(out)
		return out
	case 1:
		return ins[0]
	}

	out := make(chan A)

	var wg sync.WaitGroup
	for _, in := range ins {
		wg.Go(func() {
			for x := range in {
				out <- x
			}
		})
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

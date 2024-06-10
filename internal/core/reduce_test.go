package core

import (
	"fmt"
	"testing"

	"github.com/destel/rill/internal/th"
)

func TestReduce(t *testing.T) {
	in := th.FromRange(0, 100)

	res, _ := Reduce(in, 4, func(a, b int) int {
		return a + b
	})

	fmt.Println(res)
	fmt.Println(99 * 100 / 2)
}

func TestMapReduce(t *testing.T) {
	in := make(chan string)
	go func() {
		defer close(in)
		for i := 0; i < 100; i++ {
			th.Send(in, "foo", "bar", "baz", "foo")
		}
	}()

	out := MapReduce(in,
		4, func(a string) (string, int) {
			return a, 1
		},
		3, func(a, b int) int {
			return a + b
		},
	)

	fmt.Println(out)
}

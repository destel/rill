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

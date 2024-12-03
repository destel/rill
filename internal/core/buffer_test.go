package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/destel/rill/internal/heapbuffer"
	"github.com/destel/rill/internal/th"
)

func TestCustomBuffer(t *testing.T) {
	in := make(chan int)

	out := CustomBuffer[int](in, heapbuffer.New(5, func(item1, item2 int) bool {
		return item1 > item2
	}))

	in <- 2
	in <- 3
	in <- 6
	in <- 4
	in <- 5

	if th.SendTimeout(in, 1*time.Second, 6) {
		t.Fatal("SendTimeout should have failed")
	}

	close(in)

	for x := range out {
		fmt.Println(x)
	}

}

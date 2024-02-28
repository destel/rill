package echans

import (
	"time"

	"github.com/destel/rill/chans"
)

func Delay[A any](in <-chan A, delay time.Duration) <-chan A {
	return chans.Delay(in, delay)
}

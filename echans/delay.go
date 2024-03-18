package echans

import (
	"time"

	"github.com/destel/rill/chans"
)

// Delay postpones the delivery of items from an input channel by a specified duration, maintaining the order.
// Useful for adding delays in processing or simulating latency.
func Delay[A any](in <-chan A, delay time.Duration) <-chan A {
	return chans.Delay(in, delay)
}

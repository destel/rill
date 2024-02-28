package chans

import (
	"fmt"
	"time"

	"github.com/destel/rill/internal/ringbuffer"
)

func infiniteBuffer[A any](in <-chan A) <-chan A {
	const shrinkInterval = 60 * time.Second

	out := make(chan A)
	go func() {
		defer close(out)

		buf := ringbuffer.Buffer[A]{}

		var nextValue A
		var hasNextValue bool

		var out1 chan<- A

		shrinkTicker := time.NewTicker(shrinkInterval)
		defer shrinkTicker.Stop()
		canShrink := true

	MainLoop:
		for {
			if !hasNextValue {
				nextValue, hasNextValue = buf.Read()
			}

			if !hasNextValue {
				if in == nil {
					return
				}
				out1 = nil
			} else {
				out1 = out
			}

			select {
			case v, ok := <-in:
				if !ok {
					in = nil
					continue MainLoop
				}
				buf.Write(v)
				canShrink = canShrink && buf.CanShrink()

			case out1 <- nextValue:
				hasNextValue = false

			case <-shrinkTicker.C:
				fmt.Println("<-shrinkTicker.C")
				if canShrink {
					buf.Shrink()
				}
				canShrink = true

			}
		}
	}()

	return out
}

type delayedValue[A any] struct {
	Value  A
	SendAt time.Time
}

func Delay[A any](in <-chan A, delay time.Duration) <-chan A {
	wrapped := make(chan delayedValue[A])
	go func() {
		defer close(wrapped)
		for v := range in {
			wrapped <- delayedValue[A]{v, time.Now().Add(delay)}
		}
	}()

	// buffering is needed to freely use sleeps in the loop below
	buffered := infiniteBuffer(wrapped)

	out := make(chan A)
	go func() {
		defer close(out)
		for item := range buffered {
			sendIn := item.SendAt.Sub(time.Now())
			if sendIn > 0 {
				time.Sleep(sendIn)
			}
			out <- item.Value
		}
	}()

	return out
}

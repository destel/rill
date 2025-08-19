package core

import (
	"time"
)

type iBuffer[A any] interface {
	Read() A
	Peek() A
	Write(A)
	IsEmpty() bool
	IsFull() bool
}

type iShrinkable interface {
	Shrink() bool
}

func CustomBuffer[A any](in <-chan A, buf iBuffer[A]) <-chan A {
	out := make(chan A)

	go func() {
		defer close(out)

		// Shrinking logic:
		// If buffer implements iShrinkable interface, we'll call Shrink() method every 60 seconds
		var shrinkChan <-chan time.Time
		var shrinkable iShrinkable

		if s, ok := buf.(iShrinkable); ok {
			shrinkTicker := time.NewTicker(60 * time.Second)
			defer shrinkTicker.Stop()

			shrinkChan = shrinkTicker.C
			shrinkable = s
		}

		// Main logic.
		// High level idea: This is a simple state machine with 4 states that determined by 2 booleans:
		// - currentIn == nil
		// - currentOut == nil
		inClosed := false

		for {
			currentIn := in
			currentOut := out

			// Disable reading from the input channel if it's closed or the buffer is full
			if inClosed || buf.IsFull() {
				currentIn = nil
			}

			// Disable writing to the output channel if the buffer is empty
			var peekedValue A
			if buf.IsEmpty() {
				currentOut = nil
			} else {
				peekedValue = buf.Peek()
			}

			// Exit if there is nothing to do
			if currentIn == nil && currentOut == nil {
				return
			}

			// Do read or write operation, whatever is possible
			select {
			case v, ok := <-currentIn:
				if !ok {
					inClosed = true
					continue
				}
				buf.Write(v)

			case currentOut <- peekedValue:
				_ = buf.Read() // discard peeked value

			case <-shrinkChan:
				_ = shrinkable.Shrink()
			}

		}

	}()

	return out
}

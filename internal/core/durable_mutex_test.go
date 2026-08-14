package core

import (
	"sync"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestDurableMutex(t *testing.T) {
	t.Run("invalid unlock", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("operation did not panic")
			}
		}()

		var mu DurableMutex
		mu.Unlock()
	})

	const workers = 10
	const iterations = 100
	const holdTime = time.Second

	th.RunSynctest(t, "uncontended", func(t *testing.T) {
		var mu DurableMutex

		for range 2 {
			th.ExpectValue(t, mu.state.Load(), int32(0))
			mu.Lock()
			th.ExpectValue(t, mu.state.Load(), int32(1))
			mu.Unlock()
		}

		th.ExpectValue(t, mu.state.Load(), int32(0))
		th.ExpectValue(t, mu.wake, nil)
	})

	th.RunSynctest(t, "contended", func(t *testing.T) {
		var mu DurableMutex

		var wg sync.WaitGroup
		var sum int
		startedAt := time.Now()

		for range workers {
			wg.Go(func() {
				for range iterations {
					mu.Lock()
					time.Sleep(holdTime)
					sum++
					mu.Unlock()
				}
			})
		}

		wg.Wait()

		th.ExpectValue(t, sum, workers*iterations)
		th.ExpectValue(t, time.Since(startedAt), workers*iterations*holdTime)
		th.ExpectValue(t, mu.state.Load(), 0)
		th.ExpectValue(t, mu.wake == nil, false)
	})
}

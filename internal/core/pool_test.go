package core

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestPool(t *testing.T) {
	// The same scenario in both modes: Unsynchronized removes locking,
	// it does not change behavior.
	th.TestVariants(t, "unsynchronized", []bool{false, true}, func(t *testing.T, unsynchronized bool) {
		created := 0
		p := &Pool[*int]{
			New:            func() *int { created++; return new(int) },
			Reset:          func(v *int) { *v = 0 },
			Unsynchronized: unsynchronized,
		}

		// an empty pool creates values
		a := p.Get()
		b := p.Get()
		th.ExpectValue(t, created, 2)

		// Put resets what it takes
		*a, *b = 42, 42
		p.Put(a)
		p.Put(b)
		th.ExpectValue(t, *a, 0)
		th.ExpectValue(t, *b, 0)

		// those two come back instead of new ones
		p.Get()
		p.Get()
		th.ExpectValue(t, created, 2)

		// and the pool is empty again
		p.Get()
		th.ExpectValue(t, created, 3)
	})

	t.Run("reset is optional", func(t *testing.T) {
		p := &Pool[*int]{New: func() *int { return new(int) }}

		v := p.Get()
		*v = 42
		p.Put(v)

		th.ExpectValue(t, *p.Get(), 42)
	})

	// A value handed out must not stay reachable from the pool: the backing
	// array would keep it alive until the slot is overwritten.
	t.Run("does not retain handed out values", func(t *testing.T) {
		p := &Pool[*int]{New: func() *int { return new(int) }}

		p.Put(p.Get()) // make pool non-empty

		th.ExpectValue(t, p.items[:1][0] == nil, false)
		p.Get()
		th.ExpectValue(t, p.items[:1][0] == nil, true)
	})

	// Every worker holds its value across a sleep, so all of them are checked
	// out at once: the pool must create exactly one value per worker, and must
	// never hand the same one to two workers.
	th.RunSynctest(t, "concurrent", func(t *testing.T) {
		const workers = 8
		type value struct{ inUse bool }

		var created atomic.Int64
		p := &Pool[*value]{
			New:   func() *value { created.Add(1); return &value{} },
			Reset: func(v *value) { v.inUse = false },
		}

		var wg sync.WaitGroup
		for range workers {
			wg.Go(func() {
				for range 10 {
					v := p.Get()

					if v.inUse {
						t.Error("pool handed out a value that is already in use")
						return
					}

					v.inUse = true
					time.Sleep(1 * time.Second)

					p.Put(v)
				}
			})
		}
		wg.Wait()

		th.ExpectValue(t, created.Load(), int64(workers))
	})
}

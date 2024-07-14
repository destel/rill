package core

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

/*
// OnceWithWait is like sync.Once, but also allows waiting until the first call is complete.
type OnceWithWait struct {
	once     sync.Once
	done     chan struct{}
	initOnce sync.Once
}

func (o *OnceWithWait) init() {
	o.initOnce.Do(func() {
		o.done = make(chan struct{})
	})
}

// Do executes the function f only once, no matter how many times Do is called.
// It also signals any goroutines waiting on Wait().
func (o *OnceWithWait) Do(f func()) {
	o.once.Do(func() {
		o.init()
		f()
		close(o.done)
	})
}

// Wait blocks until the first call to Do is complete.
// It returns immediately if Do has already been called.
func (o *OnceWithWait) Wait() {
	o.init()
	<-o.done
}
*/

func TestOnceWithWait(t *testing.T) {
	t.Run("Do called once", func(t *testing.T) {
		var o OnceWithWait
		var calls int64

		for i := 0; i < 5; i++ {
			go func() {
				o.Do(func() {
					atomic.AddInt64(&calls, 1)
				})
			}()
		}

		time.Sleep(1 * time.Second)

		th.ExpectValue(t, atomic.LoadInt64(&calls), 1)
	})

	t.Run("Wait after Do", func(t *testing.T) {
		th.ExpectNotHang(t, 1*time.Second, func() {
			var o OnceWithWait
			o.Do(func() {})
			o.Wait()
		})
	})

	t.Run("Wait before Do", func(t *testing.T) {
		th.ExpectNotHang(t, 1*time.Second, func() {
			var o OnceWithWait

			go func() {
				time.Sleep(500 * time.Millisecond)
				o.Do(func() {})
			}()

			o.Wait()
		})
	})

	t.Run("Wait without Do", func(t *testing.T) {
		th.ExpectHang(t, 1*time.Second, func() {
			var o OnceWithWait
			o.Wait()
		})
	})
}

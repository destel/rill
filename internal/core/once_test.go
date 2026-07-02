package core

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/destel/rill/internal/th"
)

func TestOnceWithWait(t *testing.T) {
	t.Run("Do called once", func(t *testing.T) {
		var o OnceWithWait
		var calls atomic.Int64

		th.ExpectValue(t, o.WasCalled(), false)

		for range 5 {
			go func() {
				o.Do(func() {
					calls.Add(1)
				})
			}()
		}

		time.Sleep(1 * time.Second)

		th.ExpectValue(t, calls.Load(), 1)
		th.ExpectValue(t, o.WasCalled(), true)
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

	th.RunSynctestExpectBlock(t, "Wait without Do", func(t *testing.T) {
		var o OnceWithWait
		o.Wait()
	})
}

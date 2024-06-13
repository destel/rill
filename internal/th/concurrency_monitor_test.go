package th

import (
	"sync"
	"testing"
	"time"
)

func TestConcurrencyMonitor(t *testing.T) {
	c := NewConcurrencyMonitor(1 * time.Second)

	for k := 1; k <= 4; k++ {
		t.Run(Name("concurrency", k), func(t *testing.T) {
			c.Reset()

			var wg sync.WaitGroup
			for i := 0; i < k; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					c.Inc()
					defer c.Dec()
				}()
			}

			wg.Wait()

			ExpectValue(t, c.Max(), k)
		})
	}

}

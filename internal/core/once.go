package core

import (
	"sync"
	"sync/atomic"
)

// OnceWithWait is like sync.Once, but also allows waiting until the first call is complete.
type OnceWithWait struct {
	once     sync.Once
	done     chan struct{} // used in Wait
	fastDone int64         // used in WasCalled
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
		atomic.StoreInt64(&o.fastDone, 1)
		close(o.done)
	})
}

// Wait blocks until the first call to Do is complete.
// It returns immediately if Do has already been called.
func (o *OnceWithWait) Wait() {
	o.init()
	<-o.done
}

// WasCalled returns true if Do has been called.
func (o *OnceWithWait) WasCalled() bool {
	return atomic.LoadInt64(&o.fastDone) == 1
}

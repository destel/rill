package core

import (
	"sync"
	"sync/atomic"
)

// DurableMutex is a mutual exclusion lock whose contending callers block
// durably when used inside a [testing/synctest] bubble.
//
// Most critical sections do plain non-blocking work. For them a stock
// [sync.Mutex] is the right choice, with or without synctest.
// DurableMutex exists for one specific pattern: critical sections that do
// durably blocking operations while holding the lock -for example,
// atomically receiving values from a channel and appending them to a slice.
//
// Under synctest the stock mutex breaks this pattern: while the holder is
// durably blocked on a channel, other goroutines are non-durably blocked on the Lock() calls,
// so the bubble can neither advance time nor report a deadlock, and the test freezes.
// With DurableMutex the test runs normally, and a genuine deadlock is caught and reported by the framework.
//
// DurableMutex does not attempt to prevent starvation. Under contention,
// acquisition order depends on runtime scheduling, and the same goroutine
// can win the lock 1000 times in a row. Use it only when waiters are
// interchangeable and it does not matter which one wins the lock.
//
// A DurableMutex used within a synctest bubble is local to that bubble:
// every Lock and Unlock call on it must occur in the same bubble.
//
// The zero value is ready to use, unlocking an unlocked mutex
// panics, and a DurableMutex must not be copied after first use.
type DurableMutex struct {
	state atomic.Int32
	once  sync.Once
	wake  chan struct{}
}

func (m *DurableMutex) Lock() {
	// transition 0 -> 1: fast acquire
	if m.state.CompareAndSwap(0, 1) {
		return
	}

	m.once.Do(func() {
		m.wake = make(chan struct{}, 1)
	})

	// transition 0->2: lock acquired
	// transition 1->2, 2->2: need to park, wait for a signal, and recheck
	for m.state.Swap(2) != 0 {
		<-m.wake
	}
}

func (m *DurableMutex) Unlock() {
	// transition 1 -> 0: fast release
	if m.state.CompareAndSwap(1, 0) {
		return
	}

	// transition 0->0, 1->0: should never happen
	if m.state.Swap(0) != 2 {
		panic("rill: unlock of unlocked DurableMutex")
	}

	// 2->0: slow release, notify others.
	// m.wake is guaranteed to be initialized (we were in the state 2)
	// failed send means that one wake credit already exists, so dropping another is safe.
	SendNB(m.wake, struct{}{})
}

package pad

import (
	"sync"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/pad"

	"github.com/stretchr/testify/assert"
)

func newLockTestManager() *Manager {
	return &Manager{padLocks: newPadLocks(), store: db.NewMemoryDataStore(), globalPadCache: &GlobalPadCache{padCache: map[string]*pad.Pad{}}}
}

func TestLockPadSerialisesWriters(t *testing.T) {
	m := newLockTestManager()
	var mu sync.Mutex
	inside, maxInside := 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := mustLock(m.LockPad("pad"))
			defer unlock()
			mu.Lock()
			inside++
			if inside > maxInside {
				maxInside = inside
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			inside--
			mu.Unlock()
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, maxInside)
}

func TestRLockPadAllowsConcurrentReaders(t *testing.T) {
	m := newLockTestManager()
	unlock1 := mustLock(m.RLockPad("pad"))
	done := make(chan struct{})
	go func() {
		unlock2 := mustLock(m.RLockPad("pad"))
		unlock2()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second reader blocked behind first reader")
	}
	unlock1()
}

func TestLockPadEntriesAreReleased(t *testing.T) {
	m := newLockTestManager()
	for i := 0; i < 100; i++ {
		unlock := mustLock(m.LockPad("pad-" + string(rune('a'+i%26))))
		unlock()
		unlock() // idempotent
	}
	unlock := mustLock(m.LockPad("a", "b"))
	assert.Equal(t, 2, m.padLocks.size())
	unlock()
	assert.Equal(t, 0, m.padLocks.size(), "lock entries must not accumulate per pad")
}

func TestLockPadMultipleIdsNoDeadlock(t *testing.T) {
	m := newLockTestManager()
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); u := mustLock(m.LockPad("a", "b")); u() }()
		go func() { defer wg.Done(); u := mustLock(m.LockPad("b", "a", "b", "")); u() }()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock locking pads in opposite order")
	}
}

// mustLock returns the unlock function of a successful LockPad/RLockPad and
// panics (failing the test) otherwise.
func mustLock(unlock func(), err error) func() {
	if err != nil {
		panic(err)
	}
	return unlock
}

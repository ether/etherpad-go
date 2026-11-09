package db

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/models/db"
)

// A multi-pad lock takes its slots as a unit: it neither waits for slots it
// holds itself nor deadlocks with another multi-pad lock.
func TestLockSlotsTakeAllOrNone(t *testing.T) {
	slots := newLockSlots(2)
	var held, maxHeld atomic.Int32
	lockOne := func(ctx context.Context, name string) (func(), error) {
		n := held.Add(1)
		for {
			m := maxHeld.Load()
			if n <= m || maxHeld.CompareAndSwap(m, n) {
				break
			}
		}
		return func() { held.Add(-1) }, nil
	}

	if _, err := slots.lockAll([]string{"a", "b", "c"}, lockOne); err == nil {
		t.Fatal("more pads than slots must fail, not wait forever")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					release, err := slots.lockAll([]string{"a", "b"}, lockOne)
					if err != nil {
						t.Error(err)
						return
					}
					release()
					release() // idempotent
				}
			}()
		}
		wg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("multi-pad locks deadlocked")
	}
	if maxHeld.Load() > 2 {
		t.Fatalf("held %d locks with 2 slots", maxHeld.Load())
	}
}

// A failure while locking releases what was taken.
func TestLockSlotsReleaseOnFailure(t *testing.T) {
	slots := newLockSlots(2)
	var held atomic.Int32
	calls := 0
	_, err := slots.lockAll([]string{"a", "b"}, func(ctx context.Context, name string) (func(), error) {
		calls++
		if calls == 2 {
			return nil, context.DeadlineExceeded
		}
		held.Add(1)
		return func() { held.Add(-1) }, nil
	})
	if err == nil || held.Load() != 0 {
		t.Fatalf("err %v, still held %d", err, held.Load())
	}
	// Both slots are free again.
	release, err := slots.lockAll([]string{"x", "y"}, func(context.Context, string) (func(), error) { return func() {}, nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
}

// GetPadVersion of the memory store keeps the maxima instead of scanning.
func TestMemoryPadVersionTracksMaxima(t *testing.T) {
	m := NewMemoryDataStore()
	if err := m.CreatePad("p", newTestPadDB("p")); err != nil {
		t.Fatal(err)
	}
	for _, rev := range []int{0, 2, 1} {
		if err := m.SaveRevision("p", rev, "Z:1>0$", testAText(), testPool(), nil, 0); err != nil {
			t.Fatal(err)
		}
	}
	for _, head := range []int{0, 3} {
		if err := m.SaveChatMessage("p", head, nil, 0, "hi"); err != nil {
			t.Fatal(err)
		}
	}
	_ = m.SaveChatMessage("other", 9, nil, 0, "elsewhere")
	v, _ := m.GetPadVersion("p")
	if v.Rev != 2 || v.ChatHead != 3 {
		t.Fatalf("rev %d chat %d, want 2 and 3", v.Rev, v.ChatHead)
	}
	_ = m.RemoveRevisionsOfPad("p")
	_ = m.RemoveChat("p")
	v, _ = m.GetPadVersion("p")
	if v.Rev != -1 || v.ChatHead != -1 {
		t.Fatalf("after removal: rev %d chat %d", v.Rev, v.ChatHead)
	}
	_ = m.RemovePad("p")
	if v, _ := m.GetPadVersion("p"); v.Exists || v.Rev != -1 {
		t.Fatalf("removed pad: %+v", v)
	}
}

func newTestPadDB(id string) db.PadDB {
	return db.PadDB{ID: id, Head: -1, ChatHead: -1, SavedRevisions: []db.SavedRevision{}}
}

func testAText() db.AText  { return db.AText{Text: "\n", Attribs: "|1+1"} }
func testPool() db.RevPool { return db.RevPool{NumToAttrib: map[string][]string{}, NextNum: 0} }

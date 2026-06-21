package sheetdoc

import (
	"sync"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

// TestManagerConcurrentSubmitsConverge fires many concurrent submits at one
// document and asserts no race (run with -race) and that the persisted head
// equals the number of accepted ops, with a replayable log.
func TestManagerConcurrentSubmitsConverge(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// All submit with baseRev 0; the manager rebases each against the
			// current log. Different cells so the result is deterministic.
			_, _, _ = m.Submit("p1", sheet.Op{
				Type: sheet.OpSetCell, Sheet: DefaultSheetID,
				Row: i % 10, Col: i / 10, Raw: strptr("v"), BaseRev: 0,
			}, nil, int64(i))
		}(i)
	}
	wg.Wait()

	_, head, err := m.Snapshot("p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if head != n {
		t.Fatalf("expected head %d after %d submits, got %d", n, n, head)
	}

	// Reload from store and confirm the persisted log has all ops.
	m2 := NewManager(store)
	ops, err := m2.OpsSince("p1", 0)
	if err != nil {
		t.Fatalf("OpsSince: %v", err)
	}
	if len(ops) != n {
		t.Fatalf("persisted log length %d, want %d", len(ops), n)
	}
}

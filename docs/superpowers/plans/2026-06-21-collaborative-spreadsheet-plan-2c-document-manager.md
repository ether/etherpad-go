# Kollaborative Tabelle — Plan 2c: Sheet-Dokument-Manager

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Steps use `- [ ]`.

**Goal:** Ein nebenläufigkeitssicherer, persistierender Server-Dienst, der `lib/sheet` (Kern) und die `SheetMethods`-Persistenz (Plan 2b) zu einem Dokument-Lebenszyklus zusammenführt: laden/erzeugen, Ops einreichen (rebase+apply+persist), Snapshot für den initialen Client-State, Ops-ab-Revision für Reconnect. Vollständig headless unit-testbar (inkl. `-race`).

**Architecture:** Ein `sheetdoc.Manager` cached geladene `*sheet.Document`s pro Pad-Id. Jedes Dokument hat einen eigenen Mutex → totale Op-Ordnung pro Dokument (entspricht der Per-Pad-Goroutine des Text-Pads, hier per Mutex statt Channel — gleiche Garantie, einfacher testbar). `Submit` rebaset über `Document.Submit`, persistiert Op-Log-Eintrag + Workbook-Snapshot, gibt den rebasten Op + neue Revision zurück (für Broadcast). Beim Laden wird der Workbook-Snapshot deserialisiert und das Op-Log aus `sheet_op` rekonstruiert, sodass Rebasing veralteter Client-Ops auch nach Serverneustart funktioniert.

**Scope-Grenze (bewusst):** Dieser Plan endet am Manager-API. Die rohe WebSocket-Verdrahtung (Message-Structs `SHEET_OP`/`SHEET_VARS`, Dispatch in `lib/ws/client.go`, Broadcast-Frames, Sheet-`CLIENT_READY`) wird zu **Plan 3** gezogen, weil das Wire-Protokoll erst mit dem Frontend per Playwright-E2E verifizierbar ist. Der Manager bietet exakt die Methoden, die diese Verdrahtung dann aufruft.

**Tech Stack:** Go, `encoding/json`, `sync`. Imports: `lib/sheet`, `lib/db` (DataStore-Interface). Kein Import-Zyklus (`lib/db` kennt `lib/sheet` nicht).

**Bezug:** Spec §3/§4; Plan 2 (`lib/sheet`), Plan 2b (Persistenz).

---

## Task 1: `NewDocumentAt` im Kern (Workbook + vorhandenes Op-Log)

**Files:** modify `lib/sheet/reconcile.go`, `lib/sheet/reconcile_test.go`

- [ ] **Step 1: Failing-Test** — in `reconcile_test.go` ergänzen:
```go
func TestNewDocumentAtRebasesAgainstLoadedLog(t *testing.T) {
	// Simulate a reload: workbook already materialized, log restored.
	wb := NewWorkbook()
	wb.AddSheet("s1", "Sheet1")
	log := []Op{{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 2, BaseRev: 0}}
	wb.Apply(log[0]) // materialize workbook to head 1
	d := NewDocumentAt(wb, log)
	if d.Head() != 1 {
		t.Fatalf("head must equal len(log): got %d", d.Head())
	}
	// A stale op (baseRev 0) must rebase past the loaded insert.
	if _, err := d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 0, Raw: ptr("x"), BaseRev: 0}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if d.Workbook().SheetByID("s1").GetCell(CellRef{3, 0}).Raw != "x" {
		t.Fatal("stale op not rebased against loaded log")
	}
}
```

- [ ] **Step 2: Run — fail.** `go test ./lib/sheet/ -run TestNewDocumentAt -v`

- [ ] **Step 3: Implement** — append to `reconcile.go`:
```go
// NewDocumentAt builds a Document whose workbook is already materialized to the
// end of log; head becomes len(log). Used when loading a persisted document
// (workbook from the snapshot, log from sheet_op) so stale-op rebasing keeps
// working after a server restart.
func NewDocumentAt(wb *Workbook, log []Op) *Document {
	cp := make([]Op, len(log))
	copy(cp, log)
	return &Document{wb: wb, log: cp, head: len(cp)}
}
```

- [ ] **Step 4: Run — pass.** `go test ./lib/sheet/ -run TestNewDocumentAt -v`
- [ ] **Step 5: Commit.** `git add lib/sheet/reconcile.go lib/sheet/reconcile_test.go && git commit -m "feat(sheet): add NewDocumentAt for loading persisted documents"`

---

## Task 2: `sheetdoc.Manager` — load/create + Submit + persist

**Files:** create `lib/sheetdoc/manager.go`, `lib/sheetdoc/manager_test.go`

- [ ] **Step 1: Failing-Test** — `lib/sheetdoc/manager_test.go`:
```go
package sheetdoc

import (
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

func TestManagerSubmitAndPersist(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)

	rebased, rev, err := m.Submit("p1", sheet.Op{Type: sheet.OpSetCell, Sheet: DefaultSheetID, Row: 0, Col: 0, Raw: strptr("hi"), BaseRev: 0}, nil, 1)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if rev != 1 || rebased.Type != sheet.OpSetCell {
		t.Fatalf("unexpected rev/op: %d %+v", rev, rebased)
	}

	// A fresh manager backed by the same store must reload the persisted state.
	m2 := NewManager(store)
	snap, head, err := m2.Snapshot("p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if head != 1 {
		t.Fatalf("reloaded head: got %d", head)
	}
	wb := sheet.WorkbookFromSnapshot(snap)
	if wb.SheetByID(DefaultSheetID).GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw != "hi" {
		t.Fatal("persisted cell not reloaded")
	}
}

func TestManagerOpsSinceForReconnect(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)
	for i := 0; i < 3; i++ {
		if _, _, err := m.Submit("p1", sheet.Op{Type: sheet.OpInsertRows, Sheet: DefaultSheetID, Index: 0, Count: 1, BaseRev: i}, nil, int64(i)); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}
	ops, err := m.OpsSince("p1", 1)
	if err != nil {
		t.Fatalf("OpsSince: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops since rev 1, got %d", len(ops))
	}
}

func strptr(s string) *string { return &s }
```

- [ ] **Step 2: Run — fail.** `go test ./lib/sheetdoc/ -v`

- [ ] **Step 3: Implement** — `lib/sheetdoc/manager.go`:
```go
package sheetdoc

import (
	"encoding/json"
	"sync"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

// DefaultSheetID is the id of the single sheet created for a brand-new workbook.
const DefaultSheetID = "s1"

type entry struct {
	mu  sync.Mutex
	doc *sheet.Document
}

// Manager owns the in-memory sheet documents and serializes operations per
// document (total order), persisting each op and a workbook snapshot.
type Manager struct {
	store db.DataStore
	mu    sync.Mutex
	docs  map[string]*entry
}

func NewManager(store db.DataStore) *Manager {
	return &Manager{store: store, docs: map[string]*entry{}}
}

// load returns the cached document entry for padId, loading it from the store
// or creating a fresh single-sheet workbook on first access.
func (m *Manager) load(padId string) (*entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.docs[padId]; ok {
		return e, nil
	}
	exists, err := m.store.DoesSheetExist(padId)
	if err != nil {
		return nil, err
	}
	var doc *sheet.Document
	if exists != nil && *exists {
		sd, err := m.store.GetSheet(padId)
		if err != nil {
			return nil, err
		}
		var snap sheet.WorkbookSnapshot
		if err := json.Unmarshal([]byte(sd.Snapshot), &snap); err != nil {
			return nil, err
		}
		wb := sheet.WorkbookFromSnapshot(snap)
		opsDB, err := m.store.GetSheetOps(padId, 1, sd.Head)
		if err != nil {
			return nil, err
		}
		log := make([]sheet.Op, 0, len(*opsDB))
		for _, o := range *opsDB {
			var op sheet.Op
			if err := json.Unmarshal([]byte(o.Op), &op); err != nil {
				return nil, err
			}
			log = append(log, op)
		}
		doc = sheet.NewDocumentAt(wb, log)
	} else {
		wb := sheet.NewWorkbook()
		wb.AddSheet(DefaultSheetID, "Sheet1")
		doc = sheet.NewDocument(wb)
		snapBytes, err := json.Marshal(doc.Workbook().Snapshot())
		if err != nil {
			return nil, err
		}
		if err := m.store.SaveSheet(padId, 0, string(snapBytes)); err != nil {
			return nil, err
		}
	}
	e := &entry{doc: doc}
	m.docs[padId] = e
	return e, nil
}

// Submit rebases, applies, and persists one op, returning the rebased op (for
// broadcast) and the new head revision.
func (m *Manager) Submit(padId string, op sheet.Op, authorId *string, tsMillis int64) (sheet.Op, int, error) {
	e, err := m.load(padId)
	if err != nil {
		return sheet.Op{}, 0, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	rev, err := e.doc.Submit(op)
	if err != nil {
		return sheet.Op{}, 0, err
	}
	rebased := e.doc.Log()[rev-1]

	opBytes, err := json.Marshal(rebased)
	if err != nil {
		return sheet.Op{}, 0, err
	}
	if err := m.store.SaveSheetOp(padId, rev, string(opBytes), authorId, tsMillis); err != nil {
		return sheet.Op{}, 0, err
	}
	snapBytes, err := json.Marshal(e.doc.Workbook().Snapshot())
	if err != nil {
		return sheet.Op{}, 0, err
	}
	if err := m.store.SaveSheet(padId, rev, string(snapBytes)); err != nil {
		return sheet.Op{}, 0, err
	}
	return rebased, rev, nil
}

// Snapshot returns the current workbook snapshot and head (for the initial
// client state on connect).
func (m *Manager) Snapshot(padId string) (sheet.WorkbookSnapshot, int, error) {
	e, err := m.load(padId)
	if err != nil {
		return sheet.WorkbookSnapshot{}, 0, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.doc.Workbook().Snapshot(), e.doc.Head(), nil
}

// OpsSince returns the rebased ops applied after sinceRev (for reconnect).
func (m *Manager) OpsSince(padId string, sinceRev int) ([]sheet.Op, error) {
	e, err := m.load(padId)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	log := e.doc.Log()
	if sinceRev < 0 {
		sinceRev = 0
	}
	if sinceRev > len(log) {
		sinceRev = len(log)
	}
	out := make([]sheet.Op, len(log)-sinceRev)
	copy(out, log[sinceRev:])
	return out, nil
}
```

- [ ] **Step 4: Run — pass.** `go test ./lib/sheetdoc/ -v`
- [ ] **Step 5: Commit.** `git add lib/sheetdoc/manager.go lib/sheetdoc/manager_test.go && git commit -m "feat(sheetdoc): add concurrency-safe persisting sheet document manager"`

---

## Task 3: Concurrency / race test

**Files:** create `lib/sheetdoc/concurrency_test.go`

- [ ] **Step 1: Test** — `lib/sheetdoc/concurrency_test.go`:
```go
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
			// current log. Different cells so results are deterministic-ish.
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

	// Reload from store and confirm the log replays to the same head.
	m2 := NewManager(store)
	ops, err := m2.OpsSince("p1", 0)
	if err != nil {
		t.Fatalf("OpsSince: %v", err)
	}
	if len(ops) != n {
		t.Fatalf("persisted log length %d, want %d", len(ops), n)
	}
}
```

- [ ] **Step 2: Run with race detector.** `go test ./lib/sheetdoc/ -race -run TestManagerConcurrent -v`
Expected: PASS, no race reports.

- [ ] **Step 3: Full package + vet.** `go test ./lib/sheetdoc/ -race && go vet ./lib/sheetdoc/`
- [ ] **Step 4: Commit.** `git add lib/sheetdoc/concurrency_test.go && git commit -m "test(sheetdoc): concurrent submit race + persistence test"`

---

## Self-Review (Planner)

- **Coverage:** load-or-create (T2), Submit rebase+persist (T2), Snapshot for connect (T2), OpsSince for reconnect (T2), reload-from-store correctness (T2), concurrency/total-order (T3), restart-rebasing via NewDocumentAt (T1).
- **Placeholders:** none.
- **Type consistency:** `Manager.Submit(padId, sheet.Op, authorId *string, tsMillis int64) (sheet.Op, int, error)`, `Snapshot→(sheet.WorkbookSnapshot,int,error)`, `OpsSince→([]sheet.Op,error)`, `DefaultSheetID` used in tests and impl. `NewDocumentAt(wb, log)` matches the `Document{wb,log,head}` fields (same package, unexported access OK).
- **Honest scope:** WS wire protocol NOT implemented here; folded into Plan 3 for E2E verifiability. The manager exposes precisely the methods the future Sheet WS handler needs (Submit/Snapshot/OpsSince), mirroring how `PadMessageHandler` uses `pad.Manager`.

## Roadmap next
- **Plan 3 — Frontend + WS wire:** Sheet grid + WorkbookState + FormulaEngine (HyperFormula) + SheetCollabClient; server-side `SheetMessageHandler` (message structs, `client.go` dispatch cases, broadcast) calling `sheetdoc.Manager`; `GetTypedPad(padId,"sheet",author)` on connect; Playwright E2E (two browsers converge, reconnect, formula recompute).
- **Plan 4 — xlsx import/export.**

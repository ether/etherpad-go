# Kollaborative Tabelle — Plan 2: Kollaborations-Kern (`lib/sheet`)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Eine headless, vollständig unit-getestete Go-Bibliothek `lib/sheet`, die das Workbook-/Cell-/Style-Modell, die zellbasierten Operationen, die Index-Transformation für Struktur-Ops und die serverseitige Reconcile-Logik enthält — der korrektheitskritische Kern der kollaborativen Tabelle, beweisbar per Konvergenz-Property-Tests.

**Architecture:** Reines Go, kein DB- oder WebSocket-Bezug (analog zu `lib/changeset` als Text-Kern). Ein `Workbook` hält Sheets mit sparse `Cells` und einen dedupliziereenden `StylePool`. Ein `Op` ist eine zellbasierte Operation mit Basis-Revision. Der Server vergibt eine totale Ordnung; Ops mit veralteter Basis-Revision werden über `Transform` gegen zwischenzeitliche Struktur-Ops verschoben, bevor sie angewendet werden. Zell-Ops kommutieren (Last-Writer-Wins auf Zellebene); nur Zeilen/Spalten-Insert/Delete brauchen Index-Transformation.

**Tech Stack:** Go (Standard-Lib + `testing`), `encoding/json` für Op-Serialisierung.

**Bezug:** Spec `docs/superpowers/specs/2026-06-21-collaborative-spreadsheet-design.md` §3 (Datenmodell & Op-Format). Baut auf Plan 1 (Dokumenttyp-Fundament) auf. Persistenz und WS-Handler sind Folge-Pläne (2b/2c, Roadmap am Ende).

---

## Scope (Plan 2)

**In scope:** `Workbook`, `Sheet`, `Cell`, `StylePool`, Op-Typen, `Apply`, `Transform` (Index-Transformation), serverseitiges `Reconcile` (Op-Log + Rebase veralteter Ops), Snapshot/Klon, Konvergenz-Property-Tests.

**Op-Typen v1 (korrektheitskritischer Kern):** `setCell`, `setStyle`, `clearRange`, `insertRows`, `deleteRows`, `insertCols`, `deleteCols`. (Sheet-Verwaltung `addSheet`/`removeSheet`/`renameSheet`, `merge`/`unmerge`, `setRowProp`/`setColProp` folgen in einer späteren Iteration — sie sind nicht konvergenzkritisch und YAGNI für den ersten Kern.)

**Out of scope (Folge-Pläne):** DB-Persistenz (2b), WebSocket-Handler/Broadcast (2c), Frontend (3), xlsx (4), Formel-Berechnung (clientseitig, Plan 3).

---

## Datei-Struktur

| Datei | Verantwortung |
|-------|---------------|
| `lib/sheet/cell.go` | `Cell`, `CellRef`, `CellKind` |
| `lib/sheet/style.go` | `Style`, `StylePool` (Deduplizierung) |
| `lib/sheet/sheet.go` | `Sheet` (sparse Zellen, Struktur-Mutationen), `Workbook`, Klon/Snapshot |
| `lib/sheet/op.go` | `Op`, `OpType`, Payload, JSON-Serialisierung, Validierung |
| `lib/sheet/apply.go` | `Workbook.Apply(op)` |
| `lib/sheet/transform.go` | `Transform(incoming, applied)` Index-Transformation |
| `lib/sheet/reconcile.go` | `Document` (Op-Log + Head), `Submit(op)` Rebase + Apply |
| `lib/sheet/*_test.go` | Unit- + Property-Tests |

---

## Task 1: Cell & CellRef

**Files:** Create `lib/sheet/cell.go`, `lib/sheet/cell_test.go`

- [ ] **Step 1: Failing-Test**

`lib/sheet/cell_test.go`:
```go
package sheet

import "testing"

func TestCellIsEmpty(t *testing.T) {
	var empty Cell
	if !empty.IsEmpty() {
		t.Fatal("zero-value cell should be empty")
	}
	c := Cell{Raw: "42"}
	if c.IsEmpty() {
		t.Fatal("cell with raw should not be empty")
	}
	styled := Cell{StyleId: 3}
	if styled.IsEmpty() {
		t.Fatal("cell with style should not be empty")
	}
}

func TestCellRefComparable(t *testing.T) {
	a := CellRef{Row: 1, Col: 2}
	b := CellRef{Row: 1, Col: 2}
	if a != b {
		t.Fatal("CellRef values with equal coords must be equal (map key usable)")
	}
}
```

- [ ] **Step 2: Run — fail (undefined)**

Run: `go test ./lib/sheet/ -run TestCell -v`
Expected: FAIL build (Cell/CellRef undefined).

- [ ] **Step 3: Implement**

`lib/sheet/cell.go`:
```go
package sheet

// CellRef is a zero-based (row, col) address within a single sheet.
// It is a comparable struct so it can be used directly as a map key.
type CellRef struct {
	Row int
	Col int
}

// CellKind classifies a cell's raw content.
type CellKind string

const (
	KindValue   CellKind = "value"
	KindFormula CellKind = "formula"
)

// Cell is the atomic unit of a sheet. Raw is the source of truth (a literal
// value or a formula string like "=SUM(A1:A10)"). Value/ValueType are an
// optional client-reported cache of the computed result; the backend core
// never computes them. StyleId references the workbook StylePool.
type Cell struct {
	Raw       string `json:"raw"`
	Value     string `json:"value,omitempty"`
	ValueType string `json:"valueType,omitempty"`
	StyleId   int    `json:"styleId"`
}

// Kind reports whether the raw content is a formula (leading '=') or a value.
func (c Cell) Kind() CellKind {
	if len(c.Raw) > 0 && c.Raw[0] == '=' {
		return KindFormula
	}
	return KindValue
}

// IsEmpty reports whether the cell carries no content and default styling,
// i.e. it can be dropped from sparse storage.
func (c Cell) IsEmpty() bool {
	return c.Raw == "" && c.StyleId == 0 && c.Value == ""
}
```

- [ ] **Step 4: Run — pass**

Run: `go test ./lib/sheet/ -run TestCell -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/sheet/cell.go lib/sheet/cell_test.go
git commit -m "feat(sheet): add Cell and CellRef core types"
```

---

## Task 2: StylePool (Deduplizierung)

**Files:** Create `lib/sheet/style.go`, `lib/sheet/style_test.go`

- [ ] **Step 1: Failing-Test**

`lib/sheet/style_test.go`:
```go
package sheet

import "testing"

func TestStylePoolDedup(t *testing.T) {
	p := NewStylePool()
	id1 := p.Put(Style{Props: map[string]string{"bold": "1", "numFmt": "0.00"}})
	id2 := p.Put(Style{Props: map[string]string{"numFmt": "0.00", "bold": "1"}}) // same, different order
	if id1 != id2 {
		t.Fatalf("equal styles must dedup to same id, got %d and %d", id1, id2)
	}
	id3 := p.Put(Style{Props: map[string]string{"bold": "1"}})
	if id3 == id1 {
		t.Fatal("different styles must get different ids")
	}
}

func TestStylePoolEmptyIsZero(t *testing.T) {
	p := NewStylePool()
	if got := p.Put(Style{}); got != 0 {
		t.Fatalf("empty style must map to id 0, got %d", got)
	}
	if got := p.Put(Style{Props: map[string]string{}}); got != 0 {
		t.Fatalf("style with empty props must map to id 0, got %d", got)
	}
}

func TestStylePoolGet(t *testing.T) {
	p := NewStylePool()
	id := p.Put(Style{Props: map[string]string{"color": "#ff0000"}})
	s, ok := p.Get(id)
	if !ok || s.Props["color"] != "#ff0000" {
		t.Fatalf("Get(%d) failed: ok=%v style=%+v", id, ok, s)
	}
}
```

- [ ] **Step 2: Run — fail**

Run: `go test ./lib/sheet/ -run TestStylePool -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement**

`lib/sheet/style.go`:
```go
package sheet

import (
	"sort"
	"strings"
)

// Style is a set of formatting properties (e.g. numFmt, bold, color, align,
// border). Kept as a string->string map so the pool stays format-agnostic.
type Style struct {
	Props map[string]string `json:"props"`
}

// canonicalKey produces a deterministic key independent of map iteration order,
// so equal styles dedup regardless of insertion order.
func (s Style) canonicalKey() string {
	if len(s.Props) == 0 {
		return ""
	}
	keys := make([]string, 0, len(s.Props))
	for k := range s.Props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('\x00')
		b.WriteString(s.Props[k])
		b.WriteByte('\x01')
	}
	return b.String()
}

// StylePool deduplicates styles per workbook. Id 0 is reserved for the empty
// style; cells default to it.
type StylePool struct {
	IdToStyle map[int]Style  `json:"idToStyle"`
	keyToId   map[string]int
	NextId    int            `json:"nextId"`
}

func NewStylePool() *StylePool {
	return &StylePool{
		IdToStyle: map[int]Style{},
		keyToId:   map[string]int{"": 0},
		NextId:    1,
	}
}

// Put interns a style and returns its id (dedup by canonical key).
func (p *StylePool) Put(s Style) int {
	key := s.canonicalKey()
	if id, ok := p.keyToId[key]; ok {
		return id
	}
	id := p.NextId
	p.NextId++
	p.IdToStyle[id] = s
	p.keyToId[key] = id
	return id
}

// Get returns the style for an id.
func (p *StylePool) Get(id int) (Style, bool) {
	if id == 0 {
		return Style{}, true
	}
	s, ok := p.IdToStyle[id]
	return s, ok
}

// rebuildIndex repopulates keyToId after deserialization (json only restores
// the exported maps). Call after unmarshaling a pool.
func (p *StylePool) rebuildIndex() {
	p.keyToId = map[string]int{"": 0}
	for id, s := range p.IdToStyle {
		p.keyToId[s.canonicalKey()] = id
	}
}
```

- [ ] **Step 4: Run — pass**

Run: `go test ./lib/sheet/ -run TestStylePool -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/sheet/style.go lib/sheet/style_test.go
git commit -m "feat(sheet): add deduplicating StylePool"
```

---

## Task 3: Sheet & Workbook (sparse cells + clone)

**Files:** Create `lib/sheet/sheet.go`, `lib/sheet/sheet_test.go`

- [ ] **Step 1: Failing-Test**

`lib/sheet/sheet_test.go`:
```go
package sheet

import "testing"

func TestSheetSetGetClear(t *testing.T) {
	s := NewSheet("s1", "Sheet1")
	s.SetCell(CellRef{2, 3}, Cell{Raw: "hi"})
	if got := s.GetCell(CellRef{2, 3}); got.Raw != "hi" {
		t.Fatalf("expected hi, got %q", got.Raw)
	}
	// empty cell must not be stored
	s.SetCell(CellRef{2, 3}, Cell{})
	if _, ok := s.Cells[CellRef{2, 3}]; ok {
		t.Fatal("empty cell should be removed from sparse storage")
	}
}

func TestWorkbookCloneIsDeep(t *testing.T) {
	w := NewWorkbook()
	sh := w.AddSheet("s1", "Sheet1")
	sh.SetCell(CellRef{0, 0}, Cell{Raw: "x"})
	clone := w.Clone()
	clone.Sheets[0].SetCell(CellRef{0, 0}, Cell{Raw: "y"})
	if w.Sheets[0].GetCell(CellRef{0, 0}).Raw != "x" {
		t.Fatal("clone must not share cell storage with original")
	}
}

func TestWorkbookSheetByID(t *testing.T) {
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	if w.SheetByID("s1") == nil {
		t.Fatal("expected to find sheet s1")
	}
	if w.SheetByID("nope") != nil {
		t.Fatal("expected nil for unknown sheet")
	}
}
```

- [ ] **Step 2: Run — fail**

Run: `go test ./lib/sheet/ -run 'TestSheet|TestWorkbook' -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement**

`lib/sheet/sheet.go`:
```go
package sheet

// Sheet is a single tab: sparse cells plus structural metadata.
type Sheet struct {
	Id    string            `json:"id"`
	Name  string            `json:"name"`
	Cells map[CellRef]Cell  `json:"-"` // sparse; JSON handled by snapshot layer
}

func NewSheet(id, name string) *Sheet {
	return &Sheet{Id: id, Name: name, Cells: map[CellRef]Cell{}}
}

// SetCell stores a cell, dropping it from storage if empty (keeps it sparse).
func (s *Sheet) SetCell(ref CellRef, c Cell) {
	if c.IsEmpty() {
		delete(s.Cells, ref)
		return
	}
	s.Cells[ref] = c
}

// GetCell returns the cell at ref, or the zero Cell if unset.
func (s *Sheet) GetCell(ref CellRef) Cell {
	return s.Cells[ref]
}

func (s *Sheet) clone() *Sheet {
	cp := &Sheet{Id: s.Id, Name: s.Name, Cells: make(map[CellRef]Cell, len(s.Cells))}
	for k, v := range s.Cells {
		cp.Cells[k] = v
	}
	return cp
}

// Workbook is the full document: ordered sheets plus the shared StylePool.
type Workbook struct {
	Sheets []*Sheet   `json:"sheets"`
	Styles *StylePool `json:"styles"`
}

func NewWorkbook() *Workbook {
	return &Workbook{Sheets: []*Sheet{}, Styles: NewStylePool()}
}

func (w *Workbook) AddSheet(id, name string) *Sheet {
	s := NewSheet(id, name)
	w.Sheets = append(w.Sheets, s)
	return s
}

func (w *Workbook) SheetByID(id string) *Sheet {
	for _, s := range w.Sheets {
		if s.Id == id {
			return s
		}
	}
	return nil
}

// Clone returns a deep copy so callers can simulate clients independently.
func (w *Workbook) Clone() *Workbook {
	cp := &Workbook{
		Sheets: make([]*Sheet, len(w.Sheets)),
		Styles: w.Styles.clone(),
	}
	for i, s := range w.Sheets {
		cp.Sheets[i] = s.clone()
	}
	return cp
}
```

Add a `clone` to StylePool in `style.go`:
```go
func (p *StylePool) clone() *StylePool {
	cp := &StylePool{IdToStyle: make(map[int]Style, len(p.IdToStyle)), NextId: p.NextId}
	for id, s := range p.IdToStyle {
		cp.IdToStyle[id] = s
	}
	cp.rebuildIndex()
	return cp
}
```

- [ ] **Step 4: Run — pass**

Run: `go test ./lib/sheet/ -run 'TestSheet|TestWorkbook' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/sheet/sheet.go lib/sheet/sheet_test.go lib/sheet/style.go
git commit -m "feat(sheet): add Sheet and Workbook with deep clone"
```

---

## Task 4: Op type + JSON round-trip + validation

**Files:** Create `lib/sheet/op.go`, `lib/sheet/op_test.go`

- [ ] **Step 1: Failing-Test**

`lib/sheet/op_test.go`:
```go
package sheet

import (
	"encoding/json"
	"testing"
)

func TestOpJSONRoundTrip(t *testing.T) {
	raw := "=SUM(A1:A2)"
	ops := []Op{
		{Type: OpSetCell, Sheet: "s1", Row: 2, Col: 3, Raw: &raw},
		{Type: OpInsertRows, Sheet: "s1", Index: 5, Count: 2},
		{Type: OpClearRange, Sheet: "s1", Row: 0, Col: 0, EndRow: 3, EndCol: 3},
	}
	for _, op := range ops {
		b, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got Op
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Type != op.Type || got.Sheet != op.Sheet {
			t.Fatalf("round-trip mismatch: %+v vs %+v", got, op)
		}
	}
}

func TestOpValidate(t *testing.T) {
	if (Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 0}).Validate() == nil {
		t.Fatal("insertRows with count 0 must be invalid")
	}
	if (Op{Type: OpInsertRows, Sheet: "s1", Index: -1, Count: 1}).Validate() == nil {
		t.Fatal("negative index must be invalid")
	}
	raw := "x"
	if err := (Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: &raw}).Validate(); err != nil {
		t.Fatalf("valid setCell rejected: %v", err)
	}
	if (Op{Type: "bogus", Sheet: "s1"}).Validate() == nil {
		t.Fatal("unknown op type must be invalid")
	}
}
```

- [ ] **Step 2: Run — fail**

Run: `go test ./lib/sheet/ -run TestOp -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement**

`lib/sheet/op.go`:
```go
package sheet

import "fmt"

type OpType string

const (
	OpSetCell    OpType = "setCell"
	OpSetStyle   OpType = "setStyle"
	OpClearRange OpType = "clearRange"
	OpInsertRows OpType = "insertRows"
	OpDeleteRows OpType = "deleteRows"
	OpInsertCols OpType = "insertCols"
	OpDeleteCols OpType = "deleteCols"
)

// Op is one cell-based operation. BaseRev is the workbook revision the client
// composed it against (used by the server to rebase stale ops). Payload fields
// are optional per type.
type Op struct {
	Type    OpType `json:"type"`
	Sheet   string `json:"sheet"`
	BaseRev int    `json:"baseRev"`

	// Cell ops (setCell, setStyle) and the top-left of a range (clearRange).
	Row int `json:"row,omitempty"`
	Col int `json:"col,omitempty"`
	// Range end (inclusive) for clearRange.
	EndRow int `json:"endRow,omitempty"`
	EndCol int `json:"endCol,omitempty"`

	// setCell payload (pointers so "unset" is distinguishable from empty).
	Raw       *string `json:"raw,omitempty"`
	Value     *string `json:"value,omitempty"`
	ValueType *string `json:"valueType,omitempty"`
	// setCell + setStyle payload.
	StyleId *int `json:"styleId,omitempty"`

	// Structural ops (insert/delete rows/cols).
	Index int `json:"index,omitempty"`
	Count int `json:"count,omitempty"`
}

func (o Op) isStructural() bool {
	switch o.Type {
	case OpInsertRows, OpDeleteRows, OpInsertCols, OpDeleteCols:
		return true
	}
	return false
}

// Validate checks structural invariants independent of any workbook state.
func (o Op) Validate() error {
	if o.Sheet == "" {
		return fmt.Errorf("op missing sheet id")
	}
	switch o.Type {
	case OpSetCell:
		if o.Raw == nil && o.StyleId == nil {
			return fmt.Errorf("setCell needs raw and/or styleId")
		}
		if o.Row < 0 || o.Col < 0 {
			return fmt.Errorf("setCell negative coord")
		}
	case OpSetStyle:
		if o.StyleId == nil {
			return fmt.Errorf("setStyle needs styleId")
		}
		if o.Row < 0 || o.Col < 0 {
			return fmt.Errorf("setStyle negative coord")
		}
	case OpClearRange:
		if o.Row < 0 || o.Col < 0 || o.EndRow < o.Row || o.EndCol < o.Col {
			return fmt.Errorf("clearRange invalid bounds")
		}
	case OpInsertRows, OpDeleteRows, OpInsertCols, OpDeleteCols:
		if o.Index < 0 {
			return fmt.Errorf("%s negative index", o.Type)
		}
		if o.Count <= 0 {
			return fmt.Errorf("%s count must be > 0", o.Type)
		}
	default:
		return fmt.Errorf("unknown op type %q", o.Type)
	}
	return nil
}
```

- [ ] **Step 4: Run — pass**

Run: `go test ./lib/sheet/ -run TestOp -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/sheet/op.go lib/sheet/op_test.go
git commit -m "feat(sheet): add Op type with JSON round-trip and validation"
```

---

## Task 5: Apply (mutate workbook by op)

**Files:** Create `lib/sheet/apply.go`, `lib/sheet/apply_test.go`

- [ ] **Step 1: Failing-Test**

`lib/sheet/apply_test.go`:
```go
package sheet

import "testing"

func mkWB(t *testing.T) *Workbook {
	t.Helper()
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	return w
}

func ptr(s string) *string { return &s }

func TestApplySetCell(t *testing.T) {
	w := mkWB(t)
	if err := w.Apply(Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 1, Raw: ptr("42")}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if w.SheetByID("s1").GetCell(CellRef{1, 1}).Raw != "42" {
		t.Fatal("setCell did not store raw")
	}
}

func TestApplyClearRange(t *testing.T) {
	w := mkWB(t)
	s := w.SheetByID("s1")
	s.SetCell(CellRef{0, 0}, Cell{Raw: "a"})
	s.SetCell(CellRef{1, 1}, Cell{Raw: "b"})
	s.SetCell(CellRef{5, 5}, Cell{Raw: "keep"})
	if err := w.Apply(Op{Type: OpClearRange, Sheet: "s1", Row: 0, Col: 0, EndRow: 2, EndCol: 2}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !s.GetCell(CellRef{0, 0}).IsEmpty() || !s.GetCell(CellRef{1, 1}).IsEmpty() {
		t.Fatal("clearRange did not clear cells in range")
	}
	if s.GetCell(CellRef{5, 5}).Raw != "keep" {
		t.Fatal("clearRange cleared a cell outside the range")
	}
}

func TestApplyInsertRowsShiftsCells(t *testing.T) {
	w := mkWB(t)
	s := w.SheetByID("s1")
	s.SetCell(CellRef{3, 0}, Cell{Raw: "row3"})
	if err := w.Apply(Op{Type: OpInsertRows, Sheet: "s1", Index: 2, Count: 2}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !s.GetCell(CellRef{3, 0}).IsEmpty() {
		t.Fatal("cell at old position should have moved")
	}
	if s.GetCell(CellRef{5, 0}).Raw != "row3" {
		t.Fatalf("expected cell shifted to row 5, got %+v", s.GetCell(CellRef{5, 0}))
	}
}

func TestApplyDeleteRowsRemovesAndShifts(t *testing.T) {
	w := mkWB(t)
	s := w.SheetByID("s1")
	s.SetCell(CellRef{2, 0}, Cell{Raw: "del"})
	s.SetCell(CellRef{5, 0}, Cell{Raw: "shift"})
	if err := w.Apply(Op{Type: OpDeleteRows, Sheet: "s1", Index: 2, Count: 2}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !s.GetCell(CellRef{2, 0}).IsEmpty() {
		t.Fatal("deleted-row cell should be gone")
	}
	if s.GetCell(CellRef{3, 0}).Raw != "shift" {
		t.Fatalf("expected row5 to shift to row3, got %+v", s.GetCell(CellRef{3, 0}))
	}
}
```

- [ ] **Step 2: Run — fail**

Run: `go test ./lib/sheet/ -run TestApply -v`
Expected: FAIL (Apply undefined).

- [ ] **Step 3: Implement**

`lib/sheet/apply.go`:
```go
package sheet

import "fmt"

// Apply mutates the workbook by op. The op is assumed already rebased to the
// current revision (see reconcile.go). Cell ops are last-writer-wins; the
// caller's total ordering decides the winner.
func (w *Workbook) Apply(op Op) error {
	if err := op.Validate(); err != nil {
		return err
	}
	s := w.SheetByID(op.Sheet)
	if s == nil {
		return fmt.Errorf("apply: unknown sheet %q", op.Sheet)
	}
	switch op.Type {
	case OpSetCell:
		cur := s.GetCell(CellRef{op.Row, op.Col})
		if op.Raw != nil {
			cur.Raw = *op.Raw
			cur.Value = ""
			cur.ValueType = ""
		}
		if op.Value != nil {
			cur.Value = *op.Value
		}
		if op.ValueType != nil {
			cur.ValueType = *op.ValueType
		}
		if op.StyleId != nil {
			cur.StyleId = *op.StyleId
		}
		s.SetCell(CellRef{op.Row, op.Col}, cur)
	case OpSetStyle:
		cur := s.GetCell(CellRef{op.Row, op.Col})
		cur.StyleId = *op.StyleId
		s.SetCell(CellRef{op.Row, op.Col}, cur)
	case OpClearRange:
		for ref := range s.Cells {
			if ref.Row >= op.Row && ref.Row <= op.EndRow && ref.Col >= op.Col && ref.Col <= op.EndCol {
				delete(s.Cells, ref)
			}
		}
	case OpInsertRows:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Row >= op.Index {
				return CellRef{r.Row + op.Count, r.Col}, true
			}
			return r, true
		})
	case OpDeleteRows:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Row >= op.Index && r.Row < op.Index+op.Count {
				return r, false // drop
			}
			if r.Row >= op.Index+op.Count {
				return CellRef{r.Row - op.Count, r.Col}, true
			}
			return r, true
		})
	case OpInsertCols:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Col >= op.Index {
				return CellRef{r.Row, r.Col + op.Count}, true
			}
			return r, true
		})
	case OpDeleteCols:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Col >= op.Index && r.Col < op.Index+op.Count {
				return r, false
			}
			if r.Col >= op.Index+op.Count {
				return CellRef{r.Row, r.Col - op.Count}, true
			}
			return r, true
		})
	default:
		return fmt.Errorf("apply: unhandled op type %q", op.Type)
	}
	return nil
}
```

Add `remap` to `sheet.go`:
```go
// remap rebuilds the sparse cell map by transforming each ref. The fn returns
// the new ref and whether to keep the cell. Used by structural row/col ops.
func (s *Sheet) remap(fn func(CellRef) (CellRef, bool)) {
	next := make(map[CellRef]Cell, len(s.Cells))
	for ref, c := range s.Cells {
		nref, keep := fn(ref)
		if keep {
			next[nref] = c
		}
	}
	s.Cells = next
}
```

- [ ] **Step 4: Run — pass**

Run: `go test ./lib/sheet/ -run TestApply -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/sheet/apply.go lib/sheet/apply_test.go lib/sheet/sheet.go
git commit -m "feat(sheet): apply cell, range, and structural row/col ops"
```

---

## Task 6: Transform (index transformation for stale ops)

**Files:** Create `lib/sheet/transform.go`, `lib/sheet/transform_test.go`

The rule: given `incoming` composed against revision R, and `applied` the op that advanced the workbook from R to R+1, return `incoming'` adjusted to apply cleanly after `applied`. Only structural ops shift coordinates; cell ops shift their (row,col); two structural ops on different sheets/axes are independent.

- [ ] **Step 1: Failing-Test**

`lib/sheet/transform_test.go`:
```go
package sheet

import "testing"

func TestTransformCellAgainstInsertRows(t *testing.T) {
	applied := Op{Type: OpInsertRows, Sheet: "s1", Index: 2, Count: 3}
	in := Op{Type: OpSetCell, Sheet: "s1", Row: 4, Col: 0, Raw: ptr("x")}
	out := Transform(in, applied)
	if out.Row != 7 {
		t.Fatalf("cell below insert must shift down by 3: got row %d", out.Row)
	}
	above := Transform(Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 0, Raw: ptr("y")}, applied)
	if above.Row != 1 {
		t.Fatalf("cell above insert must not move: got row %d", above.Row)
	}
}

func TestTransformCellAgainstDeleteRows(t *testing.T) {
	applied := Op{Type: OpDeleteRows, Sheet: "s1", Index: 2, Count: 2} // deletes rows 2,3
	below := Transform(Op{Type: OpSetCell, Sheet: "s1", Row: 5, Col: 0, Raw: ptr("x")}, applied)
	if below.Row != 3 {
		t.Fatalf("cell below delete must shift up by 2: got %d", below.Row)
	}
	// A cell inside the deleted band: clamp to the deletion index (its row is gone).
	inside := Transform(Op{Type: OpSetCell, Sheet: "s1", Row: 3, Col: 0, Raw: ptr("z")}, applied)
	if inside.Row != 2 {
		t.Fatalf("cell inside deleted band should clamp to index 2: got %d", inside.Row)
	}
}

func TestTransformDifferentSheetIsNoop(t *testing.T) {
	applied := Op{Type: OpInsertRows, Sheet: "other", Index: 0, Count: 5}
	in := Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 1, Raw: ptr("x")}
	out := Transform(in, applied)
	if out.Row != 1 || out.Col != 1 {
		t.Fatal("ops on different sheets must not transform")
	}
}

func TestTransformInsertAgainstInsert(t *testing.T) {
	applied := Op{Type: OpInsertRows, Sheet: "s1", Index: 2, Count: 2}
	in := Op{Type: OpInsertRows, Sheet: "s1", Index: 4, Count: 1}
	out := Transform(in, applied)
	if out.Index != 6 {
		t.Fatalf("later insert index must shift by applied count: got %d", out.Index)
	}
}
```

- [ ] **Step 2: Run — fail**

Run: `go test ./lib/sheet/ -run TestTransform -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement**

`lib/sheet/transform.go`:
```go
package sheet

// Transform adjusts `in` so it applies cleanly after `applied`, where both were
// originally composed against the same base revision and `applied` was ordered
// first by the server. Only structural ops (row/col insert/delete) on the same
// sheet and axis move coordinates; everything else is returned unchanged.
func Transform(in, applied Op) Op {
	if in.Sheet != applied.Sheet || !applied.isStructural() {
		return in
	}
	switch applied.Type {
	case OpInsertRows:
		return shiftRows(in, applied.Index, applied.Count)
	case OpDeleteRows:
		return shiftRows(in, applied.Index, -applied.Count)
	case OpInsertCols:
		return shiftCols(in, applied.Index, applied.Count)
	case OpDeleteCols:
		return shiftCols(in, applied.Index, -applied.Count)
	}
	return in
}

// shiftRows moves row coordinates of `in` by delta for rows at/after index.
// delta > 0 is an insert; delta < 0 is a delete (band [index, index-delta)).
func shiftRows(in Op, index, delta int) Op {
	in.Row = shiftCoord(in.Row, index, delta)
	if in.Type == OpClearRange {
		in.EndRow = shiftCoord(in.EndRow, index, delta)
	}
	if in.isStructural() && (in.Type == OpInsertRows || in.Type == OpDeleteRows) {
		in.Index = shiftCoord(in.Index, index, delta)
	}
	return in
}

func shiftCols(in Op, index, delta int) Op {
	in.Col = shiftCoord(in.Col, index, delta)
	if in.Type == OpClearRange {
		in.EndCol = shiftCoord(in.EndCol, index, delta)
	}
	if in.Type == OpInsertCols || in.Type == OpDeleteCols {
		in.Index = shiftCoord(in.Index, index, delta)
	}
	return in
}

// shiftCoord shifts a single coordinate. For inserts (delta>0) coords at/after
// index move right/down. For deletes (delta<0) coords after the band move back;
// coords inside the deleted band clamp to index.
func shiftCoord(coord, index, delta int) int {
	if delta >= 0 {
		if coord >= index {
			return coord + delta
		}
		return coord
	}
	band := -delta
	if coord < index {
		return coord
	}
	if coord < index+band {
		return index // inside deleted band: clamp
	}
	return coord - band
}
```

- [ ] **Step 4: Run — pass**

Run: `go test ./lib/sheet/ -run TestTransform -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/sheet/transform.go lib/sheet/transform_test.go
git commit -m "feat(sheet): add index transformation for stale structural ops"
```

---

## Task 7: Document (server reconcile: op-log + rebase + apply)

**Files:** Create `lib/sheet/reconcile.go`, `lib/sheet/reconcile_test.go`

- [ ] **Step 1: Failing-Test**

`lib/sheet/reconcile_test.go`:
```go
package sheet

import "testing"

func newDoc(t *testing.T) *Document {
	t.Helper()
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	return NewDocument(w)
}

func TestSubmitAdvancesHead(t *testing.T) {
	d := newDoc(t)
	rev, err := d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: ptr("a"), BaseRev: 0})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if rev != 1 || d.Head() != 1 {
		t.Fatalf("expected head 1, got rev=%d head=%d", rev, d.Head())
	}
}

func TestSubmitRebasesStaleCellOp(t *testing.T) {
	d := newDoc(t)
	// Client A inserts 2 rows at index 0 (head 0 -> 1).
	if _, err := d.Submit(Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 2, BaseRev: 0}); err != nil {
		t.Fatal(err)
	}
	// Client B, still on base rev 0, sets a cell at row 1. After rebasing past
	// the insert at index 0, it must land at row 3.
	if _, err := d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 0, Raw: ptr("b"), BaseRev: 0}); err != nil {
		t.Fatal(err)
	}
	if d.Workbook().SheetByID("s1").GetCell(CellRef{3, 0}).Raw != "b" {
		t.Fatalf("stale cell op was not rebased past the insert: %+v", d.Workbook().SheetByID("s1").Cells)
	}
}

func TestConvergenceTwoClients(t *testing.T) {
	// Two clients submit interleaved ops from the same base; replaying the
	// server log on a fresh workbook must reproduce the server state.
	d := newDoc(t)
	_, _ = d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: ptr("x"), BaseRev: 0})
	_, _ = d.Submit(Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 1, BaseRev: 0})
	_, _ = d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 2, Col: 0, Raw: ptr("y"), BaseRev: 1})

	replay := NewWorkbook()
	replay.AddSheet("s1", "Sheet1")
	for _, logged := range d.Log() {
		if err := replay.Apply(logged); err != nil {
			t.Fatalf("replay apply: %v", err)
		}
	}
	if !workbooksEqual(replay, d.Workbook()) {
		t.Fatal("replaying the server op-log diverged from server state")
	}
}
```

Add a small `workbooksEqual` helper in the test file:
```go
func workbooksEqual(a, b *Workbook) bool {
	if len(a.Sheets) != len(b.Sheets) {
		return false
	}
	for i := range a.Sheets {
		if a.Sheets[i].Id != b.Sheets[i].Id {
			return false
		}
		if len(a.Sheets[i].Cells) != len(b.Sheets[i].Cells) {
			return false
		}
		for ref, c := range a.Sheets[i].Cells {
			if b.Sheets[i].Cells[ref] != c {
				return false
			}
		}
	}
	return true
}
```

- [ ] **Step 2: Run — fail**

Run: `go test ./lib/sheet/ -run 'TestSubmit|TestConvergence' -v`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement**

`lib/sheet/reconcile.go`:
```go
package sheet

import "fmt"

// Document is the authoritative server-side state for one sheet document:
// the current Workbook, a monotonically growing op-log, and the head revision.
// It is NOT goroutine-safe; the per-document serialization goroutine (plan 2c)
// provides the total order, exactly as the text pad channel does.
type Document struct {
	wb   *Workbook
	log  []Op // log[i] is the op that advanced head from i to i+1
	head int
}

func NewDocument(wb *Workbook) *Document {
	return &Document{wb: wb, log: []Op{}, head: 0}
}

func (d *Document) Head() int        { return d.head }
func (d *Document) Workbook() *Workbook { return d.wb }
func (d *Document) Log() []Op        { return d.log }

// Submit rebases an op composed against op.BaseRev past every op applied since
// then, applies it, appends the rebased op to the log, and returns the new
// head revision. The rebased op (not the original) is logged so replay is exact.
func (d *Document) Submit(op Op) (int, error) {
	if err := op.Validate(); err != nil {
		return 0, err
	}
	if op.BaseRev < 0 || op.BaseRev > d.head {
		return 0, fmt.Errorf("submit: baseRev %d out of range (head %d)", op.BaseRev, d.head)
	}
	rebased := op
	for i := op.BaseRev; i < d.head; i++ {
		rebased = Transform(rebased, d.log[i])
	}
	rebased.BaseRev = d.head
	if err := d.wb.Apply(rebased); err != nil {
		return 0, err
	}
	d.log = append(d.log, rebased)
	d.head++
	return d.head, nil
}
```

- [ ] **Step 4: Run — pass**

Run: `go test ./lib/sheet/ -run 'TestSubmit|TestConvergence' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/sheet/reconcile.go lib/sheet/reconcile_test.go
git commit -m "feat(sheet): add Document reconcile (op-log, rebase, apply)"
```

---

## Task 8: Randomized convergence property test

**Files:** Create `lib/sheet/convergence_test.go`

- [ ] **Step 1: Test schreiben**

A deterministic pseudo-random generator (no `math/rand` seed issues — use a small LCG seeded from a fixed constant so the test is reproducible) produces N ops from M simulated clients, each with a base rev sampled from `[lastSeenByClient, head]`. After submitting all, replaying the server log on a fresh workbook must equal the server workbook. Run many trials.

`lib/sheet/convergence_test.go`:
```go
package sheet

import "testing"

// lcg is a tiny deterministic RNG so trials are reproducible across runs.
type lcg struct{ state uint64 }

func (r *lcg) next() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state >> 16
}
func (r *lcg) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

func randomOp(r *lcg, baseRev int) Op {
	switch r.intn(5) {
	case 0:
		raw := "v"
		return Op{Type: OpSetCell, Sheet: "s1", Row: r.intn(8), Col: r.intn(8), Raw: &raw, BaseRev: baseRev}
	case 1:
		return Op{Type: OpInsertRows, Sheet: "s1", Index: r.intn(8), Count: 1 + r.intn(2), BaseRev: baseRev}
	case 2:
		return Op{Type: OpDeleteRows, Sheet: "s1", Index: r.intn(8), Count: 1 + r.intn(2), BaseRev: baseRev}
	case 3:
		return Op{Type: OpInsertCols, Sheet: "s1", Index: r.intn(8), Count: 1 + r.intn(2), BaseRev: baseRev}
	default:
		return Op{Type: OpDeleteCols, Sheet: "s1", Index: r.intn(8), Count: 1 + r.intn(2), BaseRev: baseRev}
	}
}

func TestConvergencePropertyManyTrials(t *testing.T) {
	for trial := 0; trial < 200; trial++ {
		r := &lcg{state: uint64(trial)*2654435761 + 1}
		w := NewWorkbook()
		w.AddSheet("s1", "Sheet1")
		d := NewDocument(w)

		const clients = 3
		seen := make([]int, clients) // each client's last-seen rev
		for step := 0; step < 30; step++ {
			c := r.intn(clients)
			base := seen[c] + r.intn(d.Head()-seen[c]+1) // somewhere in [seen[c], head]
			if base > d.Head() {
				base = d.Head()
			}
			op := randomOp(r, base)
			if _, err := d.Submit(op); err != nil {
				t.Fatalf("trial %d step %d submit: %v", trial, step, err)
			}
			seen[c] = d.Head()
		}

		// Replay the log on a fresh workbook; it must match the server state.
		replay := NewWorkbook()
		replay.AddSheet("s1", "Sheet1")
		for i, op := range d.Log() {
			if err := replay.Apply(op); err != nil {
				t.Fatalf("trial %d replay op %d: %v", trial, i, err)
			}
		}
		if !workbooksEqual(replay, d.Workbook()) {
			t.Fatalf("trial %d: replay diverged from server state", trial)
		}
	}
}
```

- [ ] **Step 2: Run — pass**

Run: `go test ./lib/sheet/ -run TestConvergenceProperty -v`
Expected: PASS (200 trials). If any trial fails, the seed `trial` reproduces it — debug `Transform`/`Apply` for that op sequence.

- [ ] **Step 3: Full package test + vet**

Run: `go test ./lib/sheet/ && go vet ./lib/sheet/`
Expected: ok, no vet issues.

- [ ] **Step 4: Commit**

```bash
git add lib/sheet/convergence_test.go
git commit -m "test(sheet): randomized convergence property test (replay == server state)"
```

---

## Self-Review (Planner)

- **Spec §3 coverage:** Workbook/Sheet/Cell (Tasks 1,3) ✓; sparse storage ✓; StylePool dedup mirroring the attribute-pool pattern (Task 2) ✓; Op format with the named op types (Task 4) ✓; LWW cell semantics + total server order (Task 5,7) ✓; index transformation for structural ops (Task 6) ✓; snapshot/replay equivalence as the convergence proof (Task 7,8) ✓. Sheet-management ops (add/remove/rename/merge/rowProp/colProp) are explicitly deferred (Scope note) — not convergence-critical.
- **Placeholder scan:** No TODO/TBD. Every step has complete code.
- **Type consistency:** `CellRef{Row,Col}`, `Cell{Raw,Value,ValueType,StyleId}`, `Op` fields, `StylePool.Put/Get`, `Workbook.Apply`, `Transform(in,applied)`, `Document.Submit/Head/Workbook/Log` are used identically across tasks. `remap` (sheet.go) is introduced in Task 5 and used only there. `workbooksEqual` is a test helper shared by Tasks 7 and 8 (defined once in reconcile_test.go; convergence_test.go relies on it being in the same package — keep both files in `package sheet`).

---

## Roadmap: verbleibende Backend-/Folge-Pläne

- **Plan 2b — Persistenz:** Migration 008 (Tabellen `sheet`, `sheet_cell`, `sheet_op` über 3 Dialekte, FKs auf `pad(id)` analog `padRev`), `SheetMethods`-Interface in `DataStore` + Implementierung in SQLiteDB/PostgresDB/MysqlDB/MemoryDataStore (Muster: SaveRevision/GetRevision), Workbook-Snapshot-Serialisierung (StylePool als JSON-Spalte; Zellen sparse in `sheet_cell`), periodische Snapshots alle 100 Ops + Replay beim Laden. Headless gegen In-Memory-SQLite testbar.
- **Plan 2c — WebSocket-Integration:** `lib/models/ws/sheetChange.go` (Wire-Op-Format), `SheetMessageHandler` mit eigenem `ChannelOperator` (Per-Doc-Goroutine, Muster `PadMessageHandler.go:58-87`), Dispatch-Case in `client.go`/`HandleMessage`, `GetTypedPad(padID,"sheet",author)` beim Sheet-`CLIENT_READY`, `UpdateSheetClients`-Broadcast (Muster `UpdatePadClients`), Präsenz/Author-Farben wiederverwenden. E2E mit zwei Browsern in Plan 3.
- **Plan 3 — Frontend-Editor** (Grid/Collab/Formeln) und **Plan 4 — xlsx** wie in Plan 1 skizziert.

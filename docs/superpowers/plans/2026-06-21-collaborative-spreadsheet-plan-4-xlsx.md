# Kollaborative Tabelle — Plan 4: xlsx Import/Export

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Steps use `- [ ]`.

**Goal:** .xlsx-Dateien serverseitig importieren (Upload → internes Workbook → persistieren) und exportieren (persistiertes Workbook → .xlsx-Download), über die Go-Bibliothek `excelize`. Headless gegen In-Memory-SQLite und per Roundtrip-Test verifizierbar.

**Architecture:** Ein neues Paket `lib/xlsx` konvertiert zwischen `.xlsx` (excelize) und `sheet.WorkbookSnapshot`/`sheet.Workbook` (Plan 2). HTTP-Handler (`POST /s/:pad/import`, `GET /s/:pad/export.xlsx`) hängen sich an den **geteilten** `sheetdoc.Manager` (über `store.Handler.SheetManager()`), sodass Import den Live-Zustand setzt und Export ihn liest. `raw` ist die Quelle der Wahrheit: eine Formelzelle hat `raw = "=…"`, eine Wertzelle `raw = "<wert>"` — das mappt direkt auf excelize `SetCellFormula`/`SetCellValue`.

**Tech Stack:** Go, `github.com/xuri/excelize/v2` (BSD-3 — neu), Fiber v3 (`FormFile`), in-memory SQLite für Tests.

**Bezug:** Spec §6; baut auf Plan 2 (`lib/sheet`), 2b (Persistenz), 2c (`sheetdoc.Manager`), 3a (WS-Broadcast) auf.

## Scope (bewusst, konsistent mit dem Modell)

**In v1:** Zellwerte + Formeln, mehrere Tabellenblätter, Blattnamen. Roundtrip Import→Export strukturell stabil.

**NICHT in v1 (das `lib/sheet`-Modell unterstützt sie noch nicht — konsistent mit Plan 2):** Styles/Formatierung, verbundene Zellen (Merges), Charts, Pivot-Tabellen, Bilder, Makros, bedingte Formatierung, Data Validation. Unbekannte Inhalte werden **verlustfrei übersprungen** (kein harter Abbruch). Sobald das Modell Merges/Styles bekommt (Folge-Plan), wird das Mapping erweitert.

> **Hinweis zu excelize-API:** Die unten genutzten Funktionen (`NewFile`, `OpenReader`, `GetSheetList`, `GetRows`, `GetCellFormula`, `SetCellValue`, `SetCellFormula`, `NewSheet`, `DeleteSheet`, `CoordinatesToCellName`, `WriteToBuffer`) sind stabile v2-APIs. Signaturen beim Implementieren gegen die installierte Version kurz verifizieren.

---

## Task 1: excelize-Dependency

- [ ] `go get github.com/xuri/excelize/v2@latest`
- [ ] `go mod tidy`
- [ ] `go build ./...` → ok. Commit: `chore(deps): add excelize for xlsx import/export`.

---

## Task 2: `lib/xlsx` Export (Workbook → .xlsx)

**Files:** create `lib/xlsx/export.go`, `lib/xlsx/export_test.go`

- [ ] **Step 1: Failing-Test** — `lib/xlsx/export_test.go`:
```go
package xlsx

import (
	"bytes"
	"testing"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

func TestExportWritesValuesAndFormulas(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("Sheet1", "Sheet1")
	s.SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "2"})
	s.SetCell(sheet.CellRef{Row: 1, Col: 0}, sheet.Cell{Raw: "3"})
	s.SetCell(sheet.CellRef{Row: 0, Col: 1}, sheet.Cell{Raw: "=SUM(A1:A2)"})

	data, err := Export(wb)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if v, _ := f.GetCellValue("Sheet1", "A1"); v != "2" {
		t.Fatalf("A1 = %q", v)
	}
	formula, _ := f.GetCellFormula("Sheet1", "B1")
	if formula != "SUM(A1:A2)" {
		t.Fatalf("B1 formula = %q", formula)
	}
}
```

- [ ] **Step 2: Run — fail.** `go test ./lib/xlsx/ -run TestExport -v`

- [ ] **Step 3: Implement** — `lib/xlsx/export.go`:
```go
package xlsx

import (
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Export renders a workbook to .xlsx bytes. raw cells starting with '=' become
// formulas; otherwise numeric-looking raw is written as a number and the rest
// as a string. Styles/merges are out of scope for v1.
func Export(wb *sheet.Workbook) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const defaultSheet = "Sheet1"
	created := map[string]bool{}

	for i, s := range wb.Sheets {
		name := s.Name
		if name == "" {
			name = s.Id
		}
		if i == 0 {
			// rename the auto-created first sheet
			if err := f.SetSheetName(defaultSheet, name); err != nil {
				return nil, err
			}
		} else if !created[name] {
			if _, err := f.NewSheet(name); err != nil {
				return nil, err
			}
		}
		created[name] = true

		for ref, cell := range s.Cells {
			axis, err := excelize.CoordinatesToCellName(ref.Col+1, ref.Row+1)
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(cell.Raw, "=") {
				if err := f.SetCellFormula(name, axis, cell.Raw[1:]); err != nil {
					return nil, err
				}
				continue
			}
			if n, err := strconv.ParseFloat(cell.Raw, 64); err == nil {
				if err := f.SetCellValue(name, axis, n); err != nil {
					return nil, err
				}
			} else if err := f.SetCellValue(name, axis, cell.Raw); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

- [ ] **Step 4: Run — pass.** Commit: `feat(xlsx): export workbook to .xlsx`.

---

## Task 3: `lib/xlsx` Import (.xlsx → WorkbookSnapshot) + roundtrip

**Files:** create `lib/xlsx/import.go`, `lib/xlsx/import_test.go`

- [ ] **Step 1: Failing-Test** — `lib/xlsx/import_test.go`:
```go
package xlsx

import (
	"bytes"
	"testing"

	"github.com/ether/etherpad-go/lib/sheet"
)

func TestImportExportRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("Data", "Data")
	s.SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "2"})
	s.SetCell(sheet.CellRef{Row: 1, Col: 0}, sheet.Cell{Raw: "3"})
	s.SetCell(sheet.CellRef{Row: 0, Col: 1}, sheet.Cell{Raw: "=SUM(A1:A2)"})

	data, err := Export(wb)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	snap, err := Import(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	got := sheet.WorkbookFromSnapshot(snap)
	sh := got.SheetByID("Data")
	if sh == nil {
		t.Fatal("sheet Data missing after roundtrip")
	}
	if sh.GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw != "2" {
		t.Fatalf("A1 = %q", sh.GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw)
	}
	if sh.GetCell(sheet.CellRef{Row: 0, Col: 1}).Raw != "=SUM(A1:A2)" {
		t.Fatalf("B1 = %q", sh.GetCell(sheet.CellRef{Row: 0, Col: 1}).Raw)
	}
}
```

- [ ] **Step 2: Run — fail.**

- [ ] **Step 3: Implement** — `lib/xlsx/import.go`:
```go
package xlsx

import (
	"io"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Import parses an .xlsx into a WorkbookSnapshot. Sheet id == sheet name. Cells
// carry raw values, or "=<formula>" when a formula is present. Styles/merges are
// skipped in v1 (logged-skip, no error).
func Import(r io.Reader) (sheet.WorkbookSnapshot, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return sheet.WorkbookSnapshot{}, err
	}
	defer f.Close()

	wb := sheet.NewWorkbook()
	for _, name := range f.GetSheetList() {
		sh := wb.AddSheet(name, name)
		rows, err := f.GetRows(name)
		if err != nil {
			return sheet.WorkbookSnapshot{}, err
		}
		for rIdx, row := range rows {
			for cIdx, val := range row {
				axis, err := excelize.CoordinatesToCellName(cIdx+1, rIdx+1)
				if err != nil {
					return sheet.WorkbookSnapshot{}, err
				}
				raw := val
				if formula, ferr := f.GetCellFormula(name, axis); ferr == nil && formula != "" {
					raw = "=" + formula
				}
				if raw == "" {
					continue
				}
				sh.SetCell(sheet.CellRef{Row: rIdx, Col: cIdx}, sheet.Cell{Raw: raw})
			}
		}
	}
	return wb.Snapshot(), nil
}
```

- [ ] **Step 4: Run — pass.** Commit: `feat(xlsx): import .xlsx to workbook snapshot with roundtrip test`.

---

## Task 4: Manager `SetWorkbook` + store `RemoveSheetOps` + handler accessors

**Files:** modify `lib/db/DataStore.go`, all 4 store impls, `lib/sheetdoc/manager.go`, `lib/ws/PadMessageHandler.go` + `lib/ws/SheetHandler.go`; tests.

- [ ] **Step 1: `RemoveSheetOps` in `SheetMethods`** (`DataStore.go`): add `RemoveSheetOps(padId string) error`. Implement in:
  - SQLite/MySQL: `DELETE FROM sheet_op WHERE id = ?` (squirrel `sq.Delete`/`mysql.Delete`).
  - Postgres: `DELETE FROM sheet_op WHERE id = $1`.
  - Memory: `delete(m.sheetOps, padId)`.
- [ ] **Step 2: `Manager.SetWorkbook`** (`manager.go`): replaces a document's state from an imported workbook (fresh head 0, ops cleared):
```go
// SetWorkbook replaces the document's workbook (e.g. from an xlsx import),
// resetting it to revision 0 with an empty op-log.
func (m *Manager) SetWorkbook(padId string, wb *sheet.Workbook) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc := sheet.NewDocument(wb)
	snapBytes, err := json.Marshal(doc.Workbook().Snapshot())
	if err != nil {
		return err
	}
	if err := m.store.RemoveSheetOps(padId); err != nil {
		return err
	}
	if err := m.store.SaveSheet(padId, 0, string(snapBytes)); err != nil {
		return err
	}
	m.docs[padId] = &entry{doc: doc}
	return nil
}
```
- [ ] **Step 3: Accessor + reload broadcast** (`PadMessageHandler`/`SheetHandler.go`):
```go
func (p *PadMessageHandler) SheetManager() *sheetdoc.Manager { return p.sheetManager }

// BroadcastSheetReload tells every client of a sheet to re-fetch its state
// (used after an xlsx import replaces the workbook).
func (p *PadMessageHandler) BroadcastSheetReload(padId string) {
	encoded, _ := json.Marshal([]any{"message", map[string]any{
		"type": "COLLABROOM", "data": map[string]any{"type": "SHEET_RELOAD"},
	}})
	for _, socket := range p.GetRoomSockets(padId) {
		socket.SafeSend(encoded)
	}
}
```
- [ ] **Step 4: Tests** — memory + SQLite `RemoveSheetOps` clears ops; `SetWorkbook` then `Snapshot` returns the imported cells at head 0; a subsequent `Submit` succeeds at rev 1 (proves ops were cleared so the write-once PK doesn't block). Commit.

---

## Task 5: HTTP handlers + routing

**Files:** create `lib/api/sheetio/init.go`, `lib/api/sheetio/handlers.go`; modify `lib/api/init.go` (add `sheetio.Init(store)`).

- [ ] **Step 1: Handlers** — `lib/api/sheetio/handlers.go`:
```go
package sheetio

import (
	"bytes"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/xlsx"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/gofiber/fiber/v3"
)

// ImportSheet handles POST /s/:pad/import (multipart "file"), replacing the
// sheet's workbook and notifying connected clients.
func ImportSheet(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("pad")
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "missing file")
		}
		file, err := fileHeader.Open()
		if err != nil {
			return err
		}
		defer file.Close()

		snap, err := xlsx.Import(file)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid xlsx: "+err.Error())
		}
		wb := sheet.WorkbookFromSnapshot(snap)
		if err := store.Handler.SheetManager().SetWorkbook(padId, wb); err != nil {
			return err
		}
		store.Handler.BroadcastSheetReload(padId)
		return c.JSON(fiber.Map{"ok": true})
	}
}

// ExportSheet handles GET /s/:pad/export.xlsx.
func ExportSheet(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("pad")
		snap, _, err := store.Handler.SheetManager().Snapshot(padId)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "sheet not found")
		}
		data, err := xlsx.Export(sheet.WorkbookFromSnapshot(snap))
		if err != nil {
			return err
		}
		c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Set("Content-Disposition", `attachment; filename="`+padId+`.xlsx"`)
		return c.Send(bytes.NewBuffer(data).Bytes())
	}
}
```
- [ ] **Step 2: Init + routing** — `lib/api/sheetio/init.go`:
```go
package sheetio

import "github.com/ether/etherpad-go/lib"

func Init(store *lib.InitStore) {
	store.C.Post("/s/:pad/import", ImportSheet(store))
	store.C.Get("/s/:pad/export.xlsx", ExportSheet(store))
}
```
Add `sheetio.Init(store)` to `lib/api/init.go` (after `io.Init(store)`).
- [ ] **Step 3: Auth** — gate both routes with the same access check the text import uses (`store.SecurityManager`), mirroring `importHandler.ImportPad`. Verify the exact CheckAccess signature and apply (reject on no write access for import; read access for export).
- [ ] **Step 4: Build** `go build ./...`; commit.

---

## Task 6: Integration test (HTTP import → export roundtrip)

**Files:** create `lib/api/sheetio/sheetio_test.go`

- [ ] Build a Fiber app with a real `*ws.PadMessageHandler` over an in-memory SQLite store (mirror `lib/test/testutils` helpers); create the pad as a sheet (`GetTypedPad`); POST a generated .xlsx (build bytes with `xlsx.Export`) to `/s/:pad/import`; assert 200; GET `/s/:pad/export.xlsx`; re-open with excelize; assert the cells/formula survived the full HTTP roundtrip. Commit.

---

## Self-Review (Planner)

- **Coverage vs spec §6:** import (T3/T5), export (T2/T5), roundtrip stability (T3/T6), values+formulas+multi-sheet (T2/T3). Charts/styles/merges explicitly out of scope, consistent with the `lib/sheet` model (no merge/style storage yet) — documented, not silently dropped.
- **Cache coherence:** import/export use the **shared** `sheetdoc.Manager` via `store.Handler.SheetManager()`, so an import is immediately visible to live editors; `RemoveSheetOps` prevents the write-once `sheet_op` PK from blocking post-import edits; `BroadcastSheetReload` notifies open clients (client handling of SHEET_RELOAD belongs to Plan 3c).
- **Placeholders:** none; excelize calls use documented stable v2 APIs (flagged for signature verification at impl time, since the dep is added in T1).
- **Type consistency:** `xlsx.Export(*sheet.Workbook) ([]byte,error)`, `xlsx.Import(io.Reader) (sheet.WorkbookSnapshot,error)`, `Manager.SetWorkbook(padId, *sheet.Workbook) error`, `RemoveSheetOps(padId) error` consistent across interface, impls, and callers. `sheet.WorkbookFromSnapshot` bridges Import↔Manager.

## Status der Gesamt-Roadmap nach Plan 4
Pläne 1, 2, 2b, 2c, 3a, 3b — implementiert. Offen zur Umsetzung: 3c (Grid-View/Bootstrap/Toolbar), 3d (Playwright-E2E), 4 (xlsx, dieser Plan). Danach ist die kollaborative Tabelle als vollständiges Feature lieferbar.

package sheetdoc

import (
	"bytes"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	dbmodel "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/xlsx"
)

// TestXlsxImportExportThroughManager exercises the real data path the HTTP
// import/export handlers orchestrate: xlsx bytes -> Import -> manager.SetWorkbook
// (persisted to SQLite, FK against a real pad row) -> reload via a fresh manager
// -> Snapshot -> Export -> re-import, asserting values and formulas survive.
func TestXlsxImportExportThroughManager(t *testing.T) {
	store, err := db.NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteDB: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// sheet/sheet_op FK references pad(id); create the pad row first.
	if err := store.CreatePad("p1", dbmodel.PadDB{
		ID:             "p1",
		DocumentType:   "sheet",
		ChatHead:       -1,
		SavedRevisions: []dbmodel.SavedRevision{},
		Pool:           dbmodel.RevPool{NumToAttrib: map[string][]string{}, NextNum: 0},
	}); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}

	// Build a workbook and turn it into .xlsx bytes (simulating an upload).
	src := sheet.NewWorkbook()
	s := src.AddSheet("Data", "Data")
	s.SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "2"})
	s.SetCell(sheet.CellRef{Row: 1, Col: 0}, sheet.Cell{Raw: "3"})
	s.SetCell(sheet.CellRef{Row: 0, Col: 1}, sheet.Cell{Raw: "=SUM(A1:A2)"})
	uploaded, err := xlsx.Export(src)
	if err != nil {
		t.Fatalf("build upload: %v", err)
	}

	// Import path: parse -> SetWorkbook (persist).
	snap, err := xlsx.Import(bytes.NewReader(uploaded))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	m := NewManager(store)
	if err := m.SetWorkbook("p1", sheet.WorkbookFromSnapshot(snap)); err != nil {
		t.Fatalf("SetWorkbook: %v", err)
	}

	// Export path from a fresh manager (proves persistence): Snapshot -> Export.
	m2 := NewManager(store)
	outSnap, head, err := m2.Snapshot("p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if head != 0 {
		t.Fatalf("expected head 0 after import, got %d", head)
	}
	downloaded, err := xlsx.Export(sheet.WorkbookFromSnapshot(outSnap))
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Re-import the downloaded file and assert fidelity.
	final, err := xlsx.Import(bytes.NewReader(downloaded))
	if err != nil {
		t.Fatalf("re-Import: %v", err)
	}
	got := sheet.WorkbookFromSnapshot(final).SheetByID("Data")
	if got == nil {
		t.Fatal("sheet Data missing after full roundtrip")
	}
	if got.GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw != "2" {
		t.Fatalf("A1 = %q", got.GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw)
	}
	if got.GetCell(sheet.CellRef{Row: 0, Col: 1}).Raw != "=SUM(A1:A2)" {
		t.Fatalf("B1 = %q", got.GetCell(sheet.CellRef{Row: 0, Col: 1}).Raw)
	}
}

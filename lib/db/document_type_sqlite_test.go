package db

import (
	"testing"

	dbmodel "github.com/ether/etherpad-go/lib/models/db"
)

func newTestSQLiteStore(t *testing.T) *SQLiteDB {
	t.Helper()
	store, err := NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteDB: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func dbmodelPadDB(id string, docType string) dbmodel.PadDB {
	return dbmodel.PadDB{
		ID:           id,
		Head:         0,
		DocumentType: docType,
		ChatHead:     -1,
		Pool: dbmodel.RevPool{
			NumToAttrib: map[string][]string{},
			NextNum:     0,
		},
		SavedRevisions: make([]dbmodel.SavedRevision, 0),
	}
}

func TestSQLiteDocumentTypePersists(t *testing.T) {
	store := newTestSQLiteStore(t)
	if err := store.CreatePad("s1", dbmodelPadDB("s1", "sheet")); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	got, err := store.GetPad("s1")
	if err != nil {
		t.Fatalf("GetPad: %v", err)
	}
	if got.DocumentType != "sheet" {
		t.Fatalf("expected sheet, got %q", got.DocumentType)
	}
}

func TestSQLiteDocumentTypeDefaultsToText(t *testing.T) {
	store := newTestSQLiteStore(t)
	if err := store.CreatePad("s2", dbmodelPadDB("s2", "text")); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	got, err := store.GetPad("s2")
	if err != nil {
		t.Fatalf("GetPad: %v", err)
	}
	if got.DocumentType != "text" {
		t.Fatalf("expected text, got %q", got.DocumentType)
	}
}

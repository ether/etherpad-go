package pad

import (
	"testing"

	datastore "github.com/ether/etherpad-go/lib/db"
	dbmodel "github.com/ether/etherpad-go/lib/models/db"
)

func TestDocumentTypeDefaultsToText(t *testing.T) {
	store := datastore.NewMemoryDataStore()
	padDB := dbmodel.PadDB{ID: "test-default"}
	if err := store.CreatePad("test-default", padDB); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	got, err := store.GetPad("test-default")
	if err != nil {
		t.Fatalf("GetPad: %v", err)
	}
	if got.DocumentType != "text" && got.DocumentType != "" {
		t.Fatalf("expected text/empty default, got %q", got.DocumentType)
	}
}

func TestDocumentTypeRoundTrip(t *testing.T) {
	store := datastore.NewMemoryDataStore()
	padDB := dbmodel.PadDB{ID: "test-sheet", DocumentType: "sheet"}
	if err := store.CreatePad("test-sheet", padDB); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	got, err := store.GetPad("test-sheet")
	if err != nil {
		t.Fatalf("GetPad: %v", err)
	}
	if got.DocumentType != "sheet" {
		t.Fatalf("expected sheet, got %q", got.DocumentType)
	}
}

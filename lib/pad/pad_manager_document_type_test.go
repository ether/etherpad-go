package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
)

func TestGetTypedPadPersistsSheetType(t *testing.T) {
	createdHooks := hooks.NewHook()
	m := NewManager(db.NewMemoryDataStore(), &createdHooks)

	p, err := m.GetTypedPad("sheet-1", "sheet", nil)
	if err != nil {
		t.Fatalf("GetTypedPad: %v", err)
	}
	if p.DocumentType != "sheet" {
		t.Fatalf("expected sheet on model, got %q", p.DocumentType)
	}

	reloaded, err := m.store.GetPad("sheet-1")
	if err != nil {
		t.Fatalf("store.GetPad: %v", err)
	}
	if reloaded.DocumentType != "sheet" {
		t.Fatalf("expected persisted sheet, got %q", reloaded.DocumentType)
	}
}

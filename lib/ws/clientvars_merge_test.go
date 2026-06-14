package ws

import (
	"encoding/json"
	"testing"

	"github.com/ether/etherpad-go/lib/models/clientVars"
)

func TestMergeClientVarsExtra_AddsKey(t *testing.T) {
	cv := &clientVars.ClientVars{}
	out, err := MergeClientVarsExtra(cv, map[string]any{"myPlugin": "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["myPlugin"] != "hi" {
		t.Fatalf("expected myPlugin key, got %v", out["myPlugin"])
	}
}

func TestMergeClientVarsExtra_TypedFieldWins(t *testing.T) {
	cv := &clientVars.ClientVars{}
	// Marshal once to discover a real top-level key the typed struct owns.
	base, _ := json.Marshal(cv)
	var m map[string]any
	_ = json.Unmarshal(base, &m)
	var ownedKey string
	for k := range m {
		ownedKey = k
		break
	}
	if ownedKey == "" {
		t.Skip("ClientVars marshals to no top-level keys")
	}

	out, err := MergeClientVarsExtra(cv, map[string]any{ownedKey: "SHOULD_NOT_OVERRIDE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[ownedKey] == "SHOULD_NOT_OVERRIDE" {
		t.Fatalf("typed field %q was clobbered by Extra", ownedKey)
	}
}

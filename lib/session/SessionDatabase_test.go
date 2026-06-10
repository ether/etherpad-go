package session

import (
	"context"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
)

func newTestSessionDatabase(t *testing.T) (Database, db.DataStore) {
	t.Helper()
	var store db.DataStore = db.NewMemoryDataStore()
	return NewSessionDatabase(&store), store
}

func TestSessionDatabaseSetGetRoundtrip(t *testing.T) {
	sessionDB, _ := newTestSessionDatabase(t)

	want := []byte(`{"user":"alice","admin":true}`)
	if err := sessionDB.Set("sid-1", want, 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, err := sessionDB.Get("sid-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("Get returned %q, want %q", got, want)
	}
}

func TestSessionDatabaseGetMissingKeyReturnsNilNil(t *testing.T) {
	sessionDB, _ := newTestSessionDatabase(t)

	got, err := sessionDB.Get("does-not-exist")
	if err != nil {
		t.Fatalf("Get of missing key returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get of missing key returned %q, want nil", got)
	}
}

func TestSessionDatabaseSetOverwritesExistingValue(t *testing.T) {
	sessionDB, _ := newTestSessionDatabase(t)

	if err := sessionDB.Set("sid-1", []byte("first"), 0); err != nil {
		t.Fatalf("first Set returned error: %v", err)
	}
	if err := sessionDB.Set("sid-1", []byte("second"), 0); err != nil {
		t.Fatalf("second Set returned error: %v", err)
	}

	got, err := sessionDB.Get("sid-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("Get returned %q, want %q", got, "second")
	}
}

func TestSessionDatabaseDeleteRemovesKey(t *testing.T) {
	sessionDB, _ := newTestSessionDatabase(t)

	if err := sessionDB.Set("sid-1", []byte("value"), 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := sessionDB.Delete("sid-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	got, err := sessionDB.Get("sid-1")
	if err != nil {
		t.Fatalf("Get after Delete returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get after Delete returned %q, want nil", got)
	}
}

func TestSessionDatabaseDeleteMissingKeyIsNoError(t *testing.T) {
	sessionDB, _ := newTestSessionDatabase(t)

	if err := sessionDB.Delete("does-not-exist"); err != nil {
		t.Fatalf("Delete of missing key returned error: %v", err)
	}
}

func TestSessionDatabasePersistsAcrossInstances(t *testing.T) {
	// Simulates a server restart: a new Database instance backed by the
	// same DataStore must still see the session.
	var store db.DataStore = db.NewMemoryDataStore()
	first := NewSessionDatabase(&store)

	want := []byte("persisted-session-data")
	if err := first.Set("sid-restart", want, time.Hour); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	second := NewSessionDatabase(&store)
	got, err := second.Get("sid-restart")
	if err != nil {
		t.Fatalf("Get on new instance returned error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("Get on new instance returned %q, want %q", got, want)
	}
}

func TestSessionDatabaseExpiredEntryIsTreatedAsMissing(t *testing.T) {
	sessionDB, store := newTestSessionDatabase(t)

	if err := sessionDB.Set("sid-exp", []byte("ephemeral"), time.Millisecond); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	got, err := sessionDB.Get("sid-exp")
	if err != nil {
		t.Fatalf("Get of expired key returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get of expired key returned %q, want nil", got)
	}

	// The expired record should have been purged from the DataStore.
	record, err := store.GetSessionById("sid-exp")
	if err != nil {
		t.Fatalf("GetSessionById returned error: %v", err)
	}
	if record != nil {
		t.Fatalf("expired session record was not removed from the DataStore: %+v", record)
	}
}

func TestSessionDatabaseUnexpiredEntryIsReturned(t *testing.T) {
	sessionDB, _ := newTestSessionDatabase(t)

	want := []byte("still-alive")
	if err := sessionDB.Set("sid-alive", want, time.Hour); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, err := sessionDB.Get("sid-alive")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("Get returned %q, want %q", got, want)
	}
}

func TestSessionDatabaseSetIgnoresEmptyKeyAndValue(t *testing.T) {
	// Per the fiber.Storage contract, empty key or value must be ignored
	// without an error.
	sessionDB, store := newTestSessionDatabase(t)

	if err := sessionDB.Set("", []byte("value"), 0); err != nil {
		t.Fatalf("Set with empty key returned error: %v", err)
	}
	if err := sessionDB.Set("sid-empty", nil, 0); err != nil {
		t.Fatalf("Set with nil value returned error: %v", err)
	}
	if err := sessionDB.Set("sid-empty", []byte{}, 0); err != nil {
		t.Fatalf("Set with empty value returned error: %v", err)
	}

	got, err := sessionDB.Get("sid-empty")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get returned %q, want nil", got)
	}

	record, err := store.GetSessionById("")
	if err != nil {
		t.Fatalf("GetSessionById returned error: %v", err)
	}
	if record != nil {
		t.Fatalf("empty key was stored in the DataStore: %+v", record)
	}
}

func TestSessionDatabaseWithContextVariants(t *testing.T) {
	sessionDB, _ := newTestSessionDatabase(t)
	ctx := context.Background()

	want := []byte("ctx-value")
	if err := sessionDB.SetWithContext(ctx, "sid-ctx", want, 0); err != nil {
		t.Fatalf("SetWithContext returned error: %v", err)
	}

	got, err := sessionDB.GetWithContext(ctx, "sid-ctx")
	if err != nil {
		t.Fatalf("GetWithContext returned error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("GetWithContext returned %q, want %q", got, want)
	}

	if err := sessionDB.DeleteWithContext(ctx, "sid-ctx"); err != nil {
		t.Fatalf("DeleteWithContext returned error: %v", err)
	}
	got, err = sessionDB.GetWithContext(ctx, "sid-ctx")
	if err != nil {
		t.Fatalf("GetWithContext after delete returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetWithContext after delete returned %q, want nil", got)
	}

	if err := sessionDB.ResetWithContext(ctx); err != nil {
		t.Fatalf("ResetWithContext returned error: %v", err)
	}
}

func TestSessionDatabaseResetAndCloseAreSafe(t *testing.T) {
	sessionDB, _ := newTestSessionDatabase(t)

	if err := sessionDB.Reset(); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}
	if err := sessionDB.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

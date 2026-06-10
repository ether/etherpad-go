package session

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	sessionmodel "github.com/ether/etherpad-go/lib/models/session"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// Database is a fiber.Storage adapter backed by the DataStore's
// `sessionstorage` table so that HTTP sessions survive server restarts.
//
// The opaque session payload handed over by fiber is base64-encoded and
// stored in the session record's Connections field; the expiration instant
// is stored as an RFC3339 string in the Expires field (an empty Expires
// means the entry never expires). Expired entries are treated as missing on
// Get and lazily purged from the DataStore.
type Database struct {
	store db.DataStore
}

// Compile-time check that Database satisfies fiber.Storage.
var _ fiber.Storage = Database{}

// GetWithContext gets the value for the given key. `nil, nil` is returned
// when the key does not exist or the entry has expired.
func (s Database) GetWithContext(_ context.Context, key string) ([]byte, error) {
	return s.Get(key)
}

// Get gets the value for the given key. `nil, nil` is returned when the key
// does not exist or the entry has expired.
func (s Database) Get(key string) ([]byte, error) {
	if key == "" {
		return nil, nil
	}

	record, err := s.store.GetSessionById(key)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	if record.Expires != "" {
		expires, err := time.Parse(time.RFC3339, record.Expires)
		if err != nil {
			// Unreadable expiry: treat the record as corrupt and drop it.
			s.purge(key)
			return nil, nil
		}
		if !expires.After(time.Now()) {
			s.purge(key)
			return nil, nil
		}
	}

	if record.Connections == "" {
		// No payload stored; treat as missing.
		return nil, nil
	}

	val, err := base64.StdEncoding.DecodeString(record.Connections)
	if err != nil {
		// Corrupt payload: drop the record and report the key as missing.
		s.purge(key)
		return nil, nil
	}
	return val, nil
}

// SetWithContext stores the given value for the given key along with an
// expiration duration. An exp of 0 means no expiration.
func (s Database) SetWithContext(_ context.Context, key string, val []byte, exp time.Duration) error {
	return s.Set(key, val, exp)
}

// Set stores the given value for the given key along with an expiration
// duration. An exp of 0 means no expiration. Empty key or value is ignored
// without an error, as required by the fiber.Storage contract.
func (s Database) Set(key string, val []byte, exp time.Duration) error {
	if key == "" || len(val) == 0 {
		return nil
	}

	var expires string
	if exp > 0 {
		expires = time.Now().Add(exp).UTC().Format(time.RFC3339)
	}

	return s.store.SetSessionById(key, sessionmodel.Session{
		Id:             key,
		OriginalMaxAge: int(exp.Milliseconds()),
		Expires:        expires,
		Connections:    base64.StdEncoding.EncodeToString(val),
	})
}

// DeleteWithContext deletes the value for the given key. It returns no error
// if the storage does not contain the key.
func (s Database) DeleteWithContext(_ context.Context, key string) error {
	return s.Delete(key)
}

// Delete deletes the value for the given key. It returns no error if the
// storage does not contain the key.
func (s Database) Delete(key string) error {
	if key == "" {
		return nil
	}

	record, err := s.store.GetSessionById(key)
	if err != nil {
		return err
	}
	if record == nil {
		// Key does not exist; per the fiber.Storage contract this is not an
		// error.
		return nil
	}
	return s.store.RemoveSessionById(key)
}

// ResetWithContext resets the storage and deletes all keys.
func (s Database) ResetWithContext(_ context.Context) error {
	return s.Reset()
}

// Reset is a no-op: the DataStore interface offers no way to enumerate or
// bulk-delete session records. Fiber only calls Reset via an explicit
// store.Reset(), which Etherpad never does.
func (s Database) Reset() error {
	return nil
}

// Close is a no-op; the underlying DataStore's lifecycle is managed by the
// server, not by the session storage adapter.
func (s Database) Close() error {
	return nil
}

// purge removes a stale or corrupt session record, logging (but otherwise
// ignoring) failures since Get must still report the key as missing.
func (s Database) purge(key string) {
	if err := s.store.RemoveSessionById(key); err != nil {
		log.Warnf("session: failed to purge expired session %s: %v", key, err)
	}
}

func NewSessionDatabase(db *db.DataStore) Database {
	return Database{store: *db}
}

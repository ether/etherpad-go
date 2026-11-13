// Package apikeys manages the keys of the HTTP API (/api). An administrator
// creates a key per service (Moodle, a backup script, ...) with a
// description, and can disable or delete it without touching the others.
// Only a SHA-256 hash of each key is stored; the key is shown once.
package apikeys

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
)

const (
	keyPrefix = "ep_"
	keyLength = 40
	// shownPrefix is how much of a key the list shows.
	shownPrefix = 10
	// MaxDescription is the longest description, in characters.
	MaxDescription = 200
	// touchEvery is how often the last use of a key is written.
	touchEvery = time.Minute
)

var (
	// ErrNoDescription is returned for an empty description.
	ErrNoDescription = errors.New("a description is required")
	// ErrDescriptionTooLong is returned for a description over MaxDescription.
	ErrDescriptionTooLong = errors.New("the description is too long")
)

// Manager creates and checks API keys.
type Manager struct {
	store db.APIKeyMethods
	now   func() time.Time

	mu      sync.Mutex
	touched map[string]time.Time
}

// NewManager returns a Manager on store.
func NewManager(store db.APIKeyMethods) *Manager {
	return &Manager{store: store, now: time.Now, touched: map[string]time.Time{}}
}

// Hash is the stored form of a key.
func Hash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func cleanDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return "", ErrNoDescription
	}
	if utf8.RuneCountInString(description) > MaxDescription {
		return "", ErrDescriptionTooLong
	}
	return description, nil
}

// Create makes a new key. The returned string is the key; it cannot be
// recovered later.
func (m *Manager) Create(description string) (string, db.APIKey, error) {
	description, err := cleanDescription(description)
	if err != nil {
		return "", db.APIKey{}, err
	}
	key := keyPrefix + utils.RandomString(keyLength)
	k, err := m.save(key, description)
	if err != nil {
		return "", db.APIKey{}, err
	}
	return key, k, nil
}

// Import stores an existing key (from an APIKEY.txt), unless it is stored
// already. It reports whether it stored it.
func (m *Manager) Import(key, description string) (bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return false, errors.New("empty key")
	}
	existing, err := m.store.GetAPIKeyByHash(Hash(key))
	if err != nil {
		return false, err
	}
	if existing != nil {
		return false, nil
	}
	_, err = m.save(key, description)
	return err == nil, err
}

func (m *Manager) save(key, description string) (db.APIKey, error) {
	shown := key
	if len(shown) > shownPrefix {
		shown = shown[:shownPrefix]
	}
	k := db.APIKey{
		ID:          utils.RandomString(16),
		Description: description,
		Prefix:      shown,
		Hash:        Hash(key),
		CreatedAt:   m.now(),
	}
	if err := m.store.CreateAPIKey(k); err != nil {
		return db.APIKey{}, err
	}
	return k, nil
}

// List returns the keys, oldest first.
func (m *Manager) List() ([]db.APIKey, error) { return m.store.ListAPIKeys() }

// SetDisabled disables or enables a key.
func (m *Manager) SetDisabled(id string, disabled bool) error {
	return m.store.SetAPIKeyDisabled(id, disabled)
}

// Delete removes a key for good.
func (m *Manager) Delete(id string) error { return m.store.DeleteAPIKey(id) }

// Check reports whether key is a stored, enabled key, and records its use.
// Keys are compared by their hash, so the comparison takes the same time
// for every wrong key.
func (m *Manager) Check(key string) (*db.APIKey, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false
	}
	k, err := m.store.GetAPIKeyByHash(Hash(key))
	if err != nil || k == nil || k.Disabled {
		return nil, false
	}
	now := m.now()
	m.mu.Lock()
	last, seen := m.touched[k.ID]
	due := !seen || now.Sub(last) >= touchEvery
	if due {
		m.touched[k.ID] = now
	}
	m.mu.Unlock()
	if due {
		_ = m.store.TouchAPIKey(k.ID, now)
		k.LastUsed = &now
	}
	return k, true
}

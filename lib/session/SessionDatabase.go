package session

import (
	"context"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/gofiber/fiber/v3"
)

type Database struct {
	store db.DataStore
}

func (s *Database) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	return s.store.GetFiberSession(key)
}

func (s *Database) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	var expiresAt int64
	if exp > 0 {
		expiresAt = time.Now().Add(exp).Unix()
	}
	return s.store.SetFiberSession(key, val, expiresAt)
}

func (s *Database) DeleteWithContext(ctx context.Context, key string) error {
	return s.store.DeleteFiberSession(key)
}

func (s *Database) ResetWithContext(ctx context.Context) error {
	return s.store.ResetFiberSessions()
}

func (s *Database) Get(key string) ([]byte, error) {
	return s.store.GetFiberSession(key)
}

func (s *Database) Set(key string, val []byte, exp time.Duration) error {
	var expiresAt int64
	if exp > 0 {
		expiresAt = time.Now().Add(exp).Unix()
	}
	return s.store.SetFiberSession(key, val, expiresAt)
}

func (s *Database) Delete(key string) error {
	return s.store.DeleteFiberSession(key)
}

func (s *Database) ResetExpirations() error {
	return s.store.CleanupExpiredFiberSessions()
}

func (s *Database) Reset() error {
	return s.store.ResetFiberSessions()
}

func (s *Database) Close() error {
	return nil
}

func NewSessionDatabase(store db.DataStore) *Database {
	return &Database{store: store}
}

var _ fiber.Storage = (*Database)(nil)

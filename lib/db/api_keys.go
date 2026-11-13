package db

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// APIKey is a key for the HTTP API (/api), created in the admin UI. Only a
// hash of the key is stored; the key itself is shown once, when it is
// created.
type APIKey struct {
	ID          string
	Description string
	// Prefix is the start of the key, to tell keys apart.
	Prefix    string
	Hash      string
	CreatedAt time.Time
	// LastUsed is nil for a key never used.
	LastUsed *time.Time
	Disabled bool
}

// ErrAPIKeyNotFound is returned for an unknown key id.
var ErrAPIKeyNotFound = errors.New("api key not found")

// APIKeyMethods store the API keys.
type APIKeyMethods interface {
	CreateAPIKey(key APIKey) error
	// ListAPIKeys returns the keys, oldest first.
	ListAPIKeys() ([]APIKey, error)
	// GetAPIKeyByHash returns the key with this hash, or nil.
	GetAPIKeyByHash(hash string) (*APIKey, error)
	SetAPIKeyDisabled(id string, disabled bool) error
	DeleteAPIKey(id string) error
	// TouchAPIKey records that the key was used at.
	TouchAPIKey(id string, at time.Time) error
}

func millis(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UnixMilli()
}

func scanAPIKey(scan func(...any) error) (*APIKey, error) {
	var k APIKey
	var created int64
	var lastUsed sql.NullInt64
	var disabled bool
	if err := scan(&k.ID, &k.Description, &k.Prefix, &k.Hash, &created, &lastUsed, &disabled); err != nil {
		return nil, err
	}
	k.CreatedAt = time.UnixMilli(created)
	if lastUsed.Valid {
		t := time.UnixMilli(lastUsed.Int64)
		k.LastUsed = &t
	}
	k.Disabled = disabled
	return &k, nil
}

const apiKeyColumns = `id, description, prefix, key_hash, created_at, last_used, disabled`

// --- MySQL and SQLite (database/sql, ? placeholders) ---

type sqlRunner interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func createAPIKeySQL(db sqlRunner, k APIKey) error {
	_, err := db.Exec(`INSERT INTO api_keys (`+apiKeyColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		k.ID, k.Description, k.Prefix, k.Hash, k.CreatedAt.UnixMilli(), millis(k.LastUsed), k.Disabled)
	return err
}

func listAPIKeysSQL(db sqlRunner) ([]APIKey, error) {
	rows, err := db.Query(`SELECT ` + apiKeyColumns + ` FROM api_keys ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		k, err := scanAPIKey(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func getAPIKeyByHashSQL(db sqlRunner, hash string) (*APIKey, error) {
	k, err := scanAPIKey(db.QueryRow(`SELECT `+apiKeyColumns+` FROM api_keys WHERE key_hash = ?`, hash).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return k, err
}

func changedOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

func setAPIKeyDisabledSQL(db sqlRunner, id string, disabled bool) error {
	// The key may already have the state; MySQL counts only changed rows.
	var found int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE id = ?`, id).Scan(&found); err != nil {
		return err
	}
	if found == 0 {
		return ErrAPIKeyNotFound
	}
	_, err := db.Exec(`UPDATE api_keys SET disabled = ? WHERE id = ?`, disabled, id)
	return err
}

func deleteAPIKeySQL(db sqlRunner, id string) error {
	return changedOne(db.Exec(`DELETE FROM api_keys WHERE id = ?`, id))
}

func touchAPIKeySQL(db sqlRunner, id string, at time.Time) error {
	_, err := db.Exec(`UPDATE api_keys SET last_used = ? WHERE id = ?`, at.UnixMilli(), id)
	return err
}

func (d MysqlDB) CreateAPIKey(k APIKey) error               { return createAPIKeySQL(d.sqlDB, k) }
func (d MysqlDB) ListAPIKeys() ([]APIKey, error)            { return listAPIKeysSQL(d.sqlDB) }
func (d MysqlDB) GetAPIKeyByHash(h string) (*APIKey, error) { return getAPIKeyByHashSQL(d.sqlDB, h) }
func (d MysqlDB) SetAPIKeyDisabled(id string, disabled bool) error {
	return setAPIKeyDisabledSQL(d.sqlDB, id, disabled)
}
func (d MysqlDB) DeleteAPIKey(id string) error               { return deleteAPIKeySQL(d.sqlDB, id) }
func (d MysqlDB) TouchAPIKey(id string, at time.Time) error  { return touchAPIKeySQL(d.sqlDB, id, at) }
func (d SQLiteDB) CreateAPIKey(k APIKey) error               { return createAPIKeySQL(d.sqlDB, k) }
func (d SQLiteDB) ListAPIKeys() ([]APIKey, error)            { return listAPIKeysSQL(d.sqlDB) }
func (d SQLiteDB) GetAPIKeyByHash(h string) (*APIKey, error) { return getAPIKeyByHashSQL(d.sqlDB, h) }
func (d SQLiteDB) SetAPIKeyDisabled(id string, disabled bool) error {
	return setAPIKeyDisabledSQL(d.sqlDB, id, disabled)
}
func (d SQLiteDB) DeleteAPIKey(id string) error              { return deleteAPIKeySQL(d.sqlDB, id) }
func (d SQLiteDB) TouchAPIKey(id string, at time.Time) error { return touchAPIKeySQL(d.sqlDB, id, at) }

// --- Postgres ---

func (d PostgresDB) CreateAPIKey(k APIKey) error {
	_, err := d.pool.Exec(context.Background(),
		`INSERT INTO api_keys (`+apiKeyColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		k.ID, k.Description, k.Prefix, k.Hash, k.CreatedAt.UnixMilli(), millis(k.LastUsed), k.Disabled)
	return err
}

func (d PostgresDB) ListAPIKeys() ([]APIKey, error) {
	rows, err := d.pool.Query(context.Background(), `SELECT `+apiKeyColumns+` FROM api_keys ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		k, err := scanAPIKey(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (d PostgresDB) GetAPIKeyByHash(hash string) (*APIKey, error) {
	k, err := scanAPIKey(d.pool.QueryRow(context.Background(),
		`SELECT `+apiKeyColumns+` FROM api_keys WHERE key_hash = $1`, hash).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return k, err
}

func (d PostgresDB) SetAPIKeyDisabled(id string, disabled bool) error {
	tag, err := d.pool.Exec(context.Background(), `UPDATE api_keys SET disabled = $1 WHERE id = $2`, disabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

func (d PostgresDB) DeleteAPIKey(id string) error {
	tag, err := d.pool.Exec(context.Background(), `DELETE FROM api_keys WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

func (d PostgresDB) TouchAPIKey(id string, at time.Time) error {
	_, err := d.pool.Exec(context.Background(), `UPDATE api_keys SET last_used = $1 WHERE id = $2`, at.UnixMilli(), id)
	return err
}

// --- Memory ---

func (m *MemoryDataStore) CreateAPIKey(k APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.apiKeys {
		if existing.ID == k.ID || existing.Hash == k.Hash {
			return ErrConflict
		}
	}
	m.apiKeys[k.ID] = k
	return nil
}

func (m *MemoryDataStore) ListAPIKeys() ([]APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]APIKey, 0, len(m.apiKeys))
	for _, k := range m.apiKeys {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *MemoryDataStore) GetAPIKeyByHash(hash string) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.apiKeys {
		if k.Hash == hash {
			found := k
			return &found, nil
		}
	}
	return nil, nil
}

func (m *MemoryDataStore) SetAPIKeyDisabled(id string, disabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.apiKeys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}
	k.Disabled = disabled
	m.apiKeys[id] = k
	return nil
}

func (m *MemoryDataStore) DeleteAPIKey(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apiKeys[id]; !ok {
		return ErrAPIKeyNotFound
	}
	delete(m.apiKeys, id)
	return nil
}

func (m *MemoryDataStore) TouchAPIKey(id string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if k, ok := m.apiKeys[id]; ok {
		k.LastUsed = &at
		m.apiKeys[id] = k
	}
	return nil
}

var (
	_ APIKeyMethods = PostgresDB{}
	_ APIKeyMethods = MysqlDB{}
	_ APIKeyMethods = SQLiteDB{}
	_ APIKeyMethods = (*MemoryDataStore)(nil)
)

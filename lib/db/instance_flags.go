package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// InstanceFlagMethods store flags of the instance: steps that ran once for
// the whole database. A flag is set once and never changed.
type InstanceFlagMethods interface {
	// GetInstanceFlag returns the value of a flag, nil when it is not set.
	GetInstanceFlag(name string) (*string, error)
	// SetInstanceFlag sets a flag unless it is set; it reports whether
	// this call set it (false: it was set, also by another instance).
	SetInstanceFlag(name, value string) (bool, error)
}

type sqlFlags struct{ db sqlRunner }

func (s sqlFlags) GetInstanceFlag(name string) (*string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM instance_flags WHERE name = ?`, name).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s sqlFlags) SetInstanceFlag(name, value string) (bool, error) {
	_, err := s.db.Exec(`INSERT INTO instance_flags (name, value, created_at) VALUES (?, ?, ?)`, name, value, time.Now().UnixMilli())
	if err != nil && isUniqueViolation(err) {
		return false, nil
	}
	return err == nil, err
}

type pgFlags struct{ d PostgresDB }

func (p pgFlags) GetInstanceFlag(name string) (*string, error) {
	var v string
	err := p.d.pool.QueryRow(context.Background(), `SELECT value FROM instance_flags WHERE name = $1`, name).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (p pgFlags) SetInstanceFlag(name, value string) (bool, error) {
	tag, err := p.d.pool.Exec(context.Background(),
		`INSERT INTO instance_flags (name, value, created_at) VALUES ($1, $2, $3) ON CONFLICT (name) DO NOTHING`,
		name, value, time.Now().UnixMilli())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (d MysqlDB) GetInstanceFlag(name string) (*string, error) {
	return sqlFlags{db: d.sqlDB}.GetInstanceFlag(name)
}
func (d MysqlDB) SetInstanceFlag(name, value string) (bool, error) {
	return sqlFlags{db: d.sqlDB}.SetInstanceFlag(name, value)
}
func (d SQLiteDB) GetInstanceFlag(name string) (*string, error) {
	return sqlFlags{db: d.sqlDB}.GetInstanceFlag(name)
}
func (d SQLiteDB) SetInstanceFlag(name, value string) (bool, error) {
	return sqlFlags{db: d.sqlDB}.SetInstanceFlag(name, value)
}
func (d PostgresDB) GetInstanceFlag(name string) (*string, error) {
	return pgFlags{d: d}.GetInstanceFlag(name)
}
func (d PostgresDB) SetInstanceFlag(name, value string) (bool, error) {
	return pgFlags{d: d}.SetInstanceFlag(name, value)
}

func (m *MemoryDataStore) GetInstanceFlag(name string) (*string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.flags[name]
	if !ok {
		return nil, nil
	}
	return &v, nil
}

func (m *MemoryDataStore) SetInstanceFlag(name, value string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.flags[name]; ok {
		return false, nil
	}
	m.flags[name] = value
	return true, nil
}

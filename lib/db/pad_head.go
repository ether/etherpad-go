package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ether/etherpad-go/lib/models/db"
)

// PadHeadMethods keep the pad row current without rewriting the text.
type PadHeadMethods interface {
	// UpdatePadHead records a new head revision and pool in the pad row.
	// It never moves the head back: another instance may have recorded a
	// newer one meanwhile.
	UpdatePadHead(padID string, head int, pool db.RevPool) error
}

func (d PostgresDB) UpdatePadHead(padID string, head int, pool db.RevPool) error {
	raw, err := json.Marshal(pool)
	if err != nil {
		return err
	}
	_, err = d.pool.Exec(context.Background(),
		`UPDATE pad SET head = $2, pool = $3, updated_at = NOW() WHERE id = $1 AND head <= $2`,
		padID, head, raw)
	return err
}

func (d MysqlDB) UpdatePadHead(padID string, head int, pool db.RevPool) error {
	raw, err := json.Marshal(pool)
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(`UPDATE pad SET head = ?, pool = ?, updated_at = NOW() WHERE id = ? AND head <= ?`,
		head, string(raw), padID, head)
	return err
}

func (d SQLiteDB) UpdatePadHead(padID string, head int, pool db.RevPool) error {
	raw, err := json.Marshal(pool)
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(`UPDATE pad SET head = ?, pool = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND head <= ?`,
		head, string(raw), padID, head)
	return err
}

func (m *MemoryDataStore) UpdatePadHead(padID string, head int, pool db.RevPool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.padStore[padID]
	if !ok || p.Head > head {
		return nil
	}
	p.Head = head
	p.Pool = pool
	now := time.Now()
	p.UpdatedAt = &now
	m.padStore[padID] = p
	return nil
}

var (
	_ PadHeadMethods = PostgresDB{}
	_ PadHeadMethods = MysqlDB{}
	_ PadHeadMethods = SQLiteDB{}
	_ PadHeadMethods = (*MemoryDataStore)(nil)
)

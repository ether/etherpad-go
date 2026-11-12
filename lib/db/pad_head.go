package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ether/etherpad-go/lib/models/db"
)

// PadHeadMethods keep the pad row current. They change only the columns
// they name: the pad row is shared by every instance, and rewriting all of
// it from one instance's copy would undo what another instance changed.
type PadHeadMethods interface {
	// UpdatePadHead records a new head revision and pool in the pad row.
	// It never moves the head back: another instance may have recorded a
	// newer one meanwhile.
	UpdatePadHead(padID string, head int, pool db.RevPool) error
	// UpdatePadHeadText is UpdatePadHead that also stores the text of a key
	// revision as the row's cached text.
	UpdatePadHeadText(padID string, head int, pool db.RevPool, text db.AText) error
	// SetPadPublicStatus changes only the public status.
	SetPadPublicStatus(padID string, public bool) error
	// SetPadSettings replaces only the pad-wide settings (JSON).
	SetPadSettings(padID string, settings string) error
	// SetPadSavedRevisions replaces only the saved revisions. Callers hold
	// the pad lock and have just reloaded them, so no change is lost.
	SetPadSavedRevisions(padID string, saved []db.SavedRevision) error
	// ClaimReadOnlyId stores candidate as the pad's read-only id unless it
	// has one already, and returns the stored id. Instances asking at the
	// same time thus all get the same id.
	ClaimReadOnlyId(padID string, candidate string) (string, error)
}

func (d PostgresDB) ClaimReadOnlyId(padID string, candidate string) (string, error) {
	ctx := context.Background()
	if _, err := d.pool.Exec(ctx, `UPDATE pad SET readonly_id = $2 WHERE id = $1 AND readonly_id IS NULL`, padID, candidate); err != nil {
		return "", err
	}
	return storedReadOnlyId(d.GetReadonlyPad(padID))
}

func (d MysqlDB) ClaimReadOnlyId(padID string, candidate string) (string, error) {
	if _, err := d.sqlDB.Exec(`UPDATE pad SET readonly_id = ? WHERE id = ? AND readonly_id IS NULL`, candidate, padID); err != nil {
		return "", err
	}
	return storedReadOnlyId(d.GetReadonlyPad(padID))
}

func (d SQLiteDB) ClaimReadOnlyId(padID string, candidate string) (string, error) {
	if _, err := d.sqlDB.Exec(`UPDATE pad SET readonly_id = ? WHERE id = ? AND readonly_id IS NULL`, candidate, padID); err != nil {
		return "", err
	}
	return storedReadOnlyId(d.GetReadonlyPad(padID))
}

func (m *MemoryDataStore) ClaimReadOnlyId(padID string, candidate string) (string, error) {
	var stored string
	err := m.updatePadRow(padID, func(p *db.PadDB) {
		if p.ReadOnlyId == nil || *p.ReadOnlyId == "" {
			id := candidate
			p.ReadOnlyId = &id
		}
		stored = *p.ReadOnlyId
	})
	return stored, err
}

func storedReadOnlyId(id *string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return *id, nil
}

func (d PostgresDB) UpdatePadHeadText(padID string, head int, pool db.RevPool, text db.AText) error {
	raw, err := json.Marshal(pool)
	if err != nil {
		return err
	}
	_, err = d.pool.Exec(context.Background(),
		`UPDATE pad SET head = $2, pool = $3, atext_text = $4, atext_attribs = $5, updated_at = NOW() WHERE id = $1 AND head <= $2`,
		padID, head, raw, text.Text, text.Attribs)
	return err
}

func (d PostgresDB) SetPadPublicStatus(padID string, public bool) error {
	return expectPadRow(d.pool.Exec(context.Background(),
		`UPDATE pad SET public_status = $2, updated_at = NOW() WHERE id = $1`, padID, public))
}

func (d PostgresDB) SetPadSettings(padID string, settings string) error {
	return expectPadRow(d.pool.Exec(context.Background(),
		`UPDATE pad SET settings = $2, updated_at = NOW() WHERE id = $1`, padID, settings))
}

func (d PostgresDB) SetPadSavedRevisions(padID string, saved []db.SavedRevision) error {
	raw, err := marshalSavedRevisions(saved)
	if err != nil {
		return err
	}
	return expectPadRow(d.pool.Exec(context.Background(),
		`UPDATE pad SET saved_revisions = $2, updated_at = NOW() WHERE id = $1`, padID, raw))
}

func (d MysqlDB) UpdatePadHeadText(padID string, head int, pool db.RevPool, text db.AText) error {
	raw, err := json.Marshal(pool)
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(`UPDATE pad SET head = ?, pool = ?, atext_text = ?, atext_attribs = ?, updated_at = NOW() WHERE id = ? AND head <= ?`,
		head, string(raw), text.Text, text.Attribs, padID, head)
	return err
}

func (d MysqlDB) SetPadPublicStatus(padID string, public bool) error {
	// MySQL reports rows changed, not matched; check existence separately.
	if _, err := d.sqlDB.Exec(`UPDATE pad SET public_status = ? WHERE id = ?`, public, padID); err != nil {
		return err
	}
	return d.padRowExists(padID)
}

func (d MysqlDB) SetPadSettings(padID string, settings string) error {
	if _, err := d.sqlDB.Exec(`UPDATE pad SET settings = ? WHERE id = ?`, settings, padID); err != nil {
		return err
	}
	return d.padRowExists(padID)
}

func (d MysqlDB) SetPadSavedRevisions(padID string, saved []db.SavedRevision) error {
	raw, err := marshalSavedRevisions(saved)
	if err != nil {
		return err
	}
	if _, err := d.sqlDB.Exec(`UPDATE pad SET saved_revisions = ? WHERE id = ?`, string(raw), padID); err != nil {
		return err
	}
	return d.padRowExists(padID)
}

func (d MysqlDB) padRowExists(padID string) error {
	var one int
	if err := d.sqlDB.QueryRow(`SELECT 1 FROM pad WHERE id = ?`, padID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New(PadDoesNotExistError)
		}
		return err
	}
	return nil
}

func (d SQLiteDB) UpdatePadHeadText(padID string, head int, pool db.RevPool, text db.AText) error {
	raw, err := json.Marshal(pool)
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(`UPDATE pad SET head = ?, pool = ?, atext_text = ?, atext_attribs = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND head <= ?`,
		head, string(raw), text.Text, text.Attribs, padID, head)
	return err
}

func (d SQLiteDB) SetPadPublicStatus(padID string, public bool) error {
	return expectSQLRow(d.sqlDB.Exec(`UPDATE pad SET public_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, public, padID))
}

func (d SQLiteDB) SetPadSettings(padID string, settings string) error {
	return expectSQLRow(d.sqlDB.Exec(`UPDATE pad SET settings = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, settings, padID))
}

func (d SQLiteDB) SetPadSavedRevisions(padID string, saved []db.SavedRevision) error {
	raw, err := marshalSavedRevisions(saved)
	if err != nil {
		return err
	}
	return expectSQLRow(d.sqlDB.Exec(`UPDATE pad SET saved_revisions = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(raw), padID))
}

func (m *MemoryDataStore) UpdatePadHeadText(padID string, head int, pool db.RevPool, text db.AText) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.padStore[padID]
	if !ok || p.Head > head {
		return nil
	}
	p.Head = head
	p.Pool = pool
	p.ATextText = text.Text
	p.ATextAttribs = text.Attribs
	now := time.Now()
	p.UpdatedAt = &now
	m.padStore[padID] = p
	return nil
}

func (m *MemoryDataStore) SetPadPublicStatus(padID string, public bool) error {
	return m.updatePadRow(padID, func(p *db.PadDB) { p.PublicStatus = public })
}

func (m *MemoryDataStore) SetPadSettings(padID string, settings string) error {
	return m.updatePadRow(padID, func(p *db.PadDB) { s := settings; p.Settings = &s })
}

func (m *MemoryDataStore) SetPadSavedRevisions(padID string, saved []db.SavedRevision) error {
	cp := append([]db.SavedRevision(nil), saved...)
	return m.updatePadRow(padID, func(p *db.PadDB) { p.SavedRevisions = cp })
}

func (m *MemoryDataStore) updatePadRow(padID string, change func(p *db.PadDB)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.padStore[padID]
	if !ok {
		return errors.New(PadDoesNotExistError)
	}
	change(&p)
	now := time.Now()
	p.UpdatedAt = &now
	m.padStore[padID] = p
	return nil
}

func marshalSavedRevisions(saved []db.SavedRevision) ([]byte, error) {
	if saved == nil {
		saved = []db.SavedRevision{}
	}
	return json.Marshal(saved)
}

func expectPadRow(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New(PadDoesNotExistError)
	}
	return nil
}

func expectSQLRow(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return errors.New(PadDoesNotExistError)
	}
	return nil
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

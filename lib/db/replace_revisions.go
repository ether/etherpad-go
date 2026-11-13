package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/models/db"
)

func checkReplacement(revisions []db.PadSingleRevision, pad db.PadDB) error {
	if len(revisions) == 0 {
		return errors.New("a pad needs at least one revision")
	}
	for i, r := range revisions {
		if r.RevNum != i {
			return fmt.Errorf("revision %d at position %d: revisions must be numbered from 0 without gaps", r.RevNum, i)
		}
	}
	if pad.Head != len(revisions)-1 {
		return fmt.Errorf("pad head %d does not match the %d revisions", pad.Head, len(revisions))
	}
	return nil
}

func revisionPool(r db.PadSingleRevision) (string, error) {
	pool := db.RevPool{}
	if r.Pool != nil {
		pool = *r.Pool
	}
	b, err := json.Marshal(pool)
	return string(b), err
}

func (m *MemoryDataStore) ReplaceRevisions(padId string, pad db.PadDB, revisions []db.PadSingleRevision) error {
	if err := checkReplacement(revisions, pad); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.padStore[padId]
	if !ok {
		return errors.New(PadDoesNotExistError)
	}
	revs := make(map[int]db.PadSingleRevision, len(revisions))
	for _, r := range revisions {
		r.PadId = padId
		r.AuthorId = copyStringPtr(r.AuthorId)
		revs[r.RevNum] = r
	}
	now := time.Now()
	pad.ID = padId
	pad.CreatedAt = existing.CreatedAt
	pad.UpdatedAt = &now
	pad.Settings = existing.Settings
	m.padRevisions[padId] = revs
	m.maxRev[padId] = len(revisions) - 1
	m.padStore[padId] = pad
	return nil
}

// replaceRevisionsSQL is the transaction for the database/sql stores.
func replaceRevisionsSQL(conn *sql.DB, builder sq.StatementBuilderType, padId string, pad db.PadDB, revisions []db.PadSingleRevision, upsertPad func(tx *sql.Tx) error) error {
	if err := checkReplacement(revisions, pad); err != nil {
		return err
	}
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pad WHERE id = ?`, padId).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errors.New(PadDoesNotExistError)
	}
	query, args, err := builder.Delete("padRev").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(query, args...); err != nil {
		return err
	}
	for _, r := range revisions {
		pool, err := revisionPool(r)
		if err != nil {
			return err
		}
		query, args, err := builder.Insert("padRev").
			Columns("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp", "pool").
			Values(padId, r.RevNum, r.Changeset, r.AText.Text, r.AText.Attribs, r.AuthorId, r.Timestamp, pool).
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return fmt.Errorf("revision %d: %w", r.RevNum, err)
		}
	}
	if err := upsertPad(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func padRowValues(pad db.PadDB) (savedRevisions, pool string, err error) {
	sr, err := json.Marshal(pad.SavedRevisions)
	if err != nil {
		return "", "", err
	}
	p, err := json.Marshal(pad.Pool)
	if err != nil {
		return "", "", err
	}
	return string(sr), string(p), nil
}

func (d SQLiteDB) ReplaceRevisions(padId string, pad db.PadDB, revisions []db.PadSingleRevision) error {
	return replaceRevisionsSQL(d.sqlDB, sq.StatementBuilder, padId, pad, revisions, func(tx *sql.Tx) error {
		sr, pool, err := padRowValues(pad)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE pad SET head = ?, saved_revisions = ?, pool = ?, atext_text = ?, atext_attribs = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			pad.Head, sr, pool, pad.ATextText, pad.ATextAttribs, padId)
		return err
	})
}

func (d MysqlDB) ReplaceRevisions(padId string, pad db.PadDB, revisions []db.PadSingleRevision) error {
	return replaceRevisionsSQL(d.sqlDB, mysql, padId, pad, revisions, func(tx *sql.Tx) error {
		sr, pool, err := padRowValues(pad)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE pad SET head = ?, saved_revisions = ?, pool = ?, atext_text = ?, atext_attribs = ? WHERE id = ?`,
			pad.Head, sr, pool, pad.ATextText, pad.ATextAttribs, padId)
		return err
	})
}

func (d PostgresDB) ReplaceRevisions(padId string, pad db.PadDB, revisions []db.PadSingleRevision) error {
	if err := checkReplacement(revisions, pad); err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM pad WHERE id = $1`, padId).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errors.New(PadDoesNotExistError)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM "padrev" WHERE id = $1`, padId); err != nil {
		return err
	}
	for _, r := range revisions {
		pool, err := revisionPool(r)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO "padrev" (id, rev, changeset, "atexttext", "atextattribs", "authorid", timestamp, pool, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`,
			padId, r.RevNum, r.Changeset, r.AText.Text, r.AText.Attribs, r.AuthorId, r.Timestamp, pool); err != nil {
			return fmt.Errorf("revision %d: %w", r.RevNum, err)
		}
	}
	sr, pool, err := padRowValues(pad)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE pad SET head = $1, saved_revisions = $2, pool = $3, atext_text = $4, atext_attribs = $5, updated_at = NOW() WHERE id = $6`,
		pad.Head, []byte(sr), []byte(pool), pad.ATextText, pad.ATextAttribs, padId); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

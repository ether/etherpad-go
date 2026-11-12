package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ether/etherpad-go/lib/models/db"
)

// execExpectingInsert runs an INSERT ... ON CONFLICT DO NOTHING and returns
// ErrConflict when it inserted nothing.
func execExpectingInsert(conn *sql.DB, query string, args ...any) error {
	res, err := conn.Exec(query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}

func (d SQLiteDB) GetPadVersion(padID string) (*PadVersion, error) {
	v := &PadVersion{}
	var saved []byte
	var public sql.NullBool
	var readOnly, settings sql.NullString
	err := d.sqlDB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pad WHERE id = ?),
		        COALESCE((SELECT MAX(rev) FROM padRev WHERE id = ?), -1),
		        COALESCE((SELECT MAX(padHead) FROM padChat WHERE padId = ?), -1),
		        (SELECT saved_revisions FROM pad WHERE id = ?),
		        (SELECT public_status FROM pad WHERE id = ?),
		        (SELECT readonly_id FROM pad WHERE id = ?),
		        (SELECT settings FROM pad WHERE id = ?)`,
		padID, padID, padID, padID, padID, padID, padID).Scan(&v.Exists, &v.Rev, &v.ChatHead, &saved, &public, &readOnly, &settings)
	if err != nil {
		return nil, err
	}
	var pub *bool
	if public.Valid {
		pub = &public.Bool
	}
	var ro *string
	if readOnly.Valid {
		ro = &readOnly.String
	}
	var st *string
	if settings.Valid {
		st = &settings.String
	}
	if v.Meta, err = scanPadMeta(v.Exists, saved, pub, ro, st); err != nil {
		return nil, err
	}
	return v, nil
}

func (d SQLiteDB) GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, error) {
	rows, err := d.sqlDB.Query(
		`SELECT rev, changeset, authorId, timestamp FROM padRev
		 WHERE id = ? AND rev >= ? AND rev <= ? ORDER BY rev ASC`,
		padID, fromRev, toRev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deltas []RevisionDelta
	for rows.Next() {
		var r RevisionDelta
		var ts sql.NullInt64
		if err := rows.Scan(&r.Rev, &r.Changeset, &r.AuthorId, &ts); err != nil {
			return nil, err
		}
		r.Timestamp = ts.Int64
		deltas = append(deltas, r)
	}
	return deltas, rows.Err()
}

func (d SQLiteDB) GetRevisionPool(padID string, rev int) (*db.RevPool, error) {
	var rawPool string
	if err := d.sqlDB.QueryRow(`SELECT pool FROM padRev WHERE id = ? AND rev = ?`, padID, rev).Scan(&rawPool); err != nil {
		return nil, fmt.Errorf("pool of revision %d: %w", rev, err)
	}
	var pool db.RevPool
	if err := json.Unmarshal([]byte(rawPool), &pool); err != nil {
		return nil, err
	}
	return &pool, nil
}

// LockPadsExclusive locks within this process: an SQLite database is used
// by one process.
func (d SQLiteDB) LockPadsExclusive(padIDs []string) (func(), error) {
	return d.locks.lockAll(padIDs), nil
}

var _ ConsistencyMethods = SQLiteDB{}

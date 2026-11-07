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
	err := d.sqlDB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pad WHERE id = ?),
		        COALESCE((SELECT MAX(rev) FROM padRev WHERE id = ?), -1),
		        COALESCE((SELECT MAX(padHead) FROM padChat WHERE padId = ?), -1)`,
		padID, padID, padID).Scan(&v.Exists, &v.Rev, &v.ChatHead)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d SQLiteDB) GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, *db.RevPool, error) {
	rows, err := d.sqlDB.Query(
		`SELECT rev, changeset, authorId, timestamp FROM padRev
		 WHERE id = ? AND rev >= ? AND rev <= ? ORDER BY rev ASC`,
		padID, fromRev, toRev)
	if err != nil {
		return nil, nil, err
	}
	var deltas []RevisionDelta
	for rows.Next() {
		var r RevisionDelta
		var ts sql.NullInt64
		if err := rows.Scan(&r.Rev, &r.Changeset, &r.AuthorId, &ts); err != nil {
			rows.Close()
			return nil, nil, err
		}
		r.Timestamp = ts.Int64
		deltas = append(deltas, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var rawPool string
	if err := d.sqlDB.QueryRow(`SELECT pool FROM padRev WHERE id = ? AND rev = ?`, padID, toRev).Scan(&rawPool); err != nil {
		return nil, nil, fmt.Errorf("pool of revision %d: %w", toRev, err)
	}
	var pool db.RevPool
	if err := json.Unmarshal([]byte(rawPool), &pool); err != nil {
		return nil, nil, err
	}
	return deltas, &pool, nil
}

// LockPadExclusive locks within this process: an SQLite database is used by
// one process.
func (d SQLiteDB) LockPadExclusive(padID string) (func(), error) {
	return d.locks.lock(padID), nil
}

var _ ConsistencyMethods = SQLiteDB{}

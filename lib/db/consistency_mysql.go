package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ether/etherpad-go/lib/models/db"
	mysql2 "github.com/go-sql-driver/mysql"
)

// isDuplicateKey reports whether err is MySQL's ER_DUP_ENTRY.
func isDuplicateKey(err error) bool {
	var me *mysql2.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func (d MysqlDB) GetPadVersion(padID string) (*PadVersion, error) {
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

func (d MysqlDB) GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, error) {
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

func (d MysqlDB) GetRevisionPool(padID string, rev int) (*db.RevPool, error) {
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

func (d MysqlDB) LockPadsExclusive(padIDs []string) (func(), error) {
	return d.lockSlots.lockAll(padLockNames(padIDs), d.lockOne)
}

// lockOne takes GET_LOCK on a connection of the lock pool. The lock belongs
// to that connection and is released when it closes, also when the process
// dies.
func (d MysqlDB) lockOne(ctx context.Context, name string) (func(), error) {
	conn, err := d.lockDB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	lockName := mysqlLockName(name)
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, ?)`, lockName, int(lockTimeout.Seconds())).Scan(&got); err != nil || got.Int64 != 1 {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		conn.Close()
		if err == nil {
			err = errors.New("timeout")
		}
		return nil, fmt.Errorf("GET_LOCK: %w", err)
	}
	return func() {
		if _, err := conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK(?)`, lockName); err != nil {
			// Still held by this connection: discard it to release.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		conn.Close()
	}, nil
}

var _ ConsistencyMethods = MysqlDB{}

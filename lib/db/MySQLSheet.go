package db

import (
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/models/db"
)

func (d MysqlDB) SaveSheet(padId string, head int, snapshot string) error {
	q, args, err := mysql.Insert("sheet").
		Columns("id", "head", "snapshot").
		Values(padId, head, snapshot).
		Suffix("ON DUPLICATE KEY UPDATE head = VALUES(head), snapshot = VALUES(snapshot)").
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d MysqlDB) GetSheet(padId string) (*db.SheetDB, error) {
	q, args, err := mysql.Select("id", "head", "snapshot", "created_at", "updated_at").
		From("sheet").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return nil, err
	}
	var s db.SheetDB
	err = d.sqlDB.QueryRow(q, args...).Scan(&s.ID, &s.Head, &s.Snapshot, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New(SheetDoesNotExistError)
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d MysqlDB) DoesSheetExist(padId string) (*bool, error) {
	q, args, err := mysql.Select("1").From("sheet").Where(sq.Eq{"id": padId}).Limit(1).ToSql()
	if err != nil {
		return nil, err
	}
	var x int
	err = d.sqlDB.QueryRow(q, args...).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		f := false
		return &f, nil
	}
	if err != nil {
		return nil, err
	}
	tr := true
	return &tr, nil
}

func (d MysqlDB) RemoveSheet(padId string) error {
	q, args, err := mysql.Delete("sheet").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d MysqlDB) SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error {
	q, args, err := mysql.Insert("sheet_op").
		Columns("id", "rev", "op", "author_id", "timestamp").
		Values(padId, rev, op, authorId, timestamp).
		Suffix("ON DUPLICATE KEY UPDATE id = id").
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d MysqlDB) GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error) {
	q, args, err := mysql.Select("id", "rev", "op", "author_id", "timestamp").
		From("sheet_op").
		Where(sq.Eq{"id": padId}).
		Where(sq.GtOrEq{"rev": startRev}).
		Where(sq.LtOrEq{"rev": endRev}).
		OrderBy("rev ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := d.sqlDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]db.SheetOpDB, 0)
	for rows.Next() {
		var o db.SheetOpDB
		if err := rows.Scan(&o.PadId, &o.Rev, &o.Op, &o.AuthorId, &o.Timestamp); err != nil {
			return nil, fmt.Errorf("scan sheet_op: %w", err)
		}
		out = append(out, o)
	}
	return &out, rows.Err()
}

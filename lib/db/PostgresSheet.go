package db

import (
	"context"
	"errors"

	"github.com/ether/etherpad-go/lib/models/db"
	"github.com/jackc/pgx/v5"
)

func (d PostgresDB) SaveSheet(padId string, head int, snapshot string) error {
	_, err := d.pool.Exec(context.Background(),
		`INSERT INTO sheet (id, head, snapshot, created_at, updated_at)
         VALUES ($1, $2, $3, NOW(), NOW())
         ON CONFLICT (id) DO UPDATE SET head = EXCLUDED.head, snapshot = EXCLUDED.snapshot, updated_at = NOW()`,
		padId, head, snapshot)
	return err
}

func (d PostgresDB) GetSheet(padId string) (*db.SheetDB, error) {
	var s db.SheetDB
	err := d.pool.QueryRow(context.Background(),
		`SELECT id, head, snapshot, created_at, updated_at FROM sheet WHERE id = $1`, padId).
		Scan(&s.ID, &s.Head, &s.Snapshot, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New(SheetDoesNotExistError)
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d PostgresDB) DoesSheetExist(padId string) (*bool, error) {
	var exists bool
	err := d.pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM sheet WHERE id = $1)`, padId).Scan(&exists)
	if err != nil {
		return nil, err
	}
	return &exists, nil
}

func (d PostgresDB) RemoveSheet(padId string) error {
	_, err := d.pool.Exec(context.Background(), `DELETE FROM sheet WHERE id = $1`, padId)
	return err
}

func (d PostgresDB) SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error {
	tag, err := d.pool.Exec(context.Background(),
		`INSERT INTO sheet_op (id, rev, op, author_id, timestamp, created_at)
         VALUES ($1, $2, $3, $4, $5, NOW()) ON CONFLICT (id, rev) DO NOTHING`,
		padId, rev, op, authorId, timestamp)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (d PostgresDB) GetSheetVersion(padId string) (*SheetVersion, error) {
	v := &SheetVersion{Exists: true}
	err := d.pool.QueryRow(context.Background(),
		`SELECT head, epoch, COALESCE((SELECT MAX(rev) FROM sheet_op WHERE id = $1), 0) FROM sheet WHERE id = $1`,
		padId).Scan(&v.Head, &v.Epoch, &v.OpHead)
	if errors.Is(err, pgx.ErrNoRows) {
		return &SheetVersion{}, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d PostgresDB) ResetSheet(padId string, snapshot string) error {
	ctx := context.Background()
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM sheet_op WHERE id = $1`, padId); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO sheet (id, head, snapshot, epoch, created_at, updated_at) VALUES ($1, 0, $2, 1, NOW(), NOW())
		 ON CONFLICT (id) DO UPDATE SET head = 0, snapshot = EXCLUDED.snapshot, epoch = sheet.epoch + 1, updated_at = NOW()`,
		padId, snapshot); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d PostgresDB) RemoveSheetOps(padId string) error {
	_, err := d.pool.Exec(context.Background(), `DELETE FROM sheet_op WHERE id = $1`, padId)
	return err
}

func (d PostgresDB) GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error) {
	rows, err := d.pool.Query(context.Background(),
		`SELECT id, rev, op, author_id, timestamp FROM sheet_op
         WHERE id = $1 AND rev >= $2 AND rev <= $3 ORDER BY rev ASC`,
		padId, startRev, endRev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]db.SheetOpDB, 0)
	for rows.Next() {
		var o db.SheetOpDB
		if err := rows.Scan(&o.PadId, &o.Rev, &o.Op, &o.AuthorId, &o.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return &out, rows.Err()
}

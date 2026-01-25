package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/db/migrations"
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

type PostgresDB struct {
	options PostgresOptions
	pool    *pgxpool.Pool
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// ============== PAD METHODS ==============

func (d PostgresDB) CreatePad(padID string, padDB db.PadDB) error {
	ctx := context.Background()

	savedRevisions, err := json.Marshal(padDB.SavedRevisions)
	if err != nil {
		return fmt.Errorf("error marshaling saved revisions: %w", err)
	}

	pool, err := json.Marshal(padDB.Pool)
	if err != nil {
		return fmt.Errorf("error marshaling pool: %w", err)
	}

	_, err = d.pool.Exec(ctx,
		`INSERT INTO pad (id, head, saved_revisions, readonly_id, pool, chat_head, 
                          public_status, atext_text, atext_attribs, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
         ON CONFLICT (id) DO UPDATE SET
             head = EXCLUDED.head,
             saved_revisions = EXCLUDED.saved_revisions,
             readonly_id = EXCLUDED.readonly_id,
             pool = EXCLUDED.pool,
             chat_head = EXCLUDED.chat_head,
             public_status = EXCLUDED.public_status,
             atext_text = EXCLUDED.atext_text,
             atext_attribs = EXCLUDED.atext_attribs,
             updated_at = NOW()`,
		padID, padDB.Head, savedRevisions, padDB.ReadOnlyId, pool,
		padDB.ChatHead, padDB.PublicStatus, padDB.ATextText, padDB.ATextAttribs)
	return err
}

func (d PostgresDB) GetPad(padID string) (*db.PadDB, error) {
	ctx := context.Background()

	padDB, err := ReadToPadDB(d.pool.QueryRow(ctx,
		`SELECT id, head, saved_revisions, readonly_id, pool, chat_head, 
                public_status, atext_text, atext_attribs, created_at, updated_at
         FROM pad WHERE id = $1`,
		padID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(PadDoesNotExistError)
		}
		return nil, err
	}

	return padDB, nil
}

func (d PostgresDB) DoesPadExist(padID string) (*bool, error) {
	ctx := context.Background()
	var exists bool
	err := d.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pad WHERE id = $1)`,
		padID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	return &exists, nil
}

func (d PostgresDB) RemovePad(padID string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx, `DELETE FROM pad WHERE id = $1`, padID)
	return err
}

func (d PostgresDB) GetPadIds() (*[]string, error) {
	ctx := context.Background()

	rows, err := d.pool.Query(ctx, `SELECT id FROM pad`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var padIds []string
	for rows.Next() {
		var padId string
		if err := rows.Scan(&padId); err != nil {
			return nil, err
		}
		padIds = append(padIds, strings.TrimPrefix(padId, "pad:"))
	}

	return &padIds, rows.Err()
}

func (d PostgresDB) SaveChatHeadOfPad(padId string, head int) error {
	ctx := context.Background()
	result, err := d.pool.Exec(ctx,
		`UPDATE pad SET chat_head = $1, updated_at = NOW() WHERE id = $2`,
		head, padId)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(PadDoesNotExistError)
	}
	return nil
}

// ============== READONLY METHODS ==============

func (d PostgresDB) GetReadonlyPad(padId string) (*string, error) {
	ctx := context.Background()

	var readonlyId *string
	err := d.pool.QueryRow(ctx,
		`SELECT readonly_id FROM pad WHERE id = $1`,
		padId).Scan(&readonlyId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(PadDoesNotExistError)
		}
		return nil, err
	}
	if readonlyId == nil {
		return nil, errors.New(PadReadOnlyIdNotFoundError)
	}
	return readonlyId, nil
}

func (d PostgresDB) SetReadOnlyId(padId string, readonlyId string) error {
	ctx := context.Background()
	result, err := d.pool.Exec(ctx,
		`UPDATE pad SET readonly_id = $1, updated_at = NOW() WHERE id = $2`,
		readonlyId, padId)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(PadDoesNotExistError)
	}
	return nil
}

func (d PostgresDB) GetPadByReadOnlyId(readonlyId string) (*string, error) {
	ctx := context.Background()

	var padId string
	err := d.pool.QueryRow(ctx,
		`SELECT id FROM pad WHERE readonly_id = $1`,
		readonlyId).Scan(&padId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &padId, nil
}

// ============== AUTHOR METHODS ==============

func (d PostgresDB) GetPadIdsOfAuthor(authorId string) (*[]string, error) {
	ctx := context.Background()
	rows, err := d.pool.Query(ctx,
		`SELECT DISTINCT pr.id
		 FROM padrev pr
		 WHERE pr.authorid = $1`,
		authorId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var padIds []string
	for rows.Next() {
		var padId string
		if err := rows.Scan(&padId); err != nil {
			return nil, err
		}
		padIds = append(padIds, padId)
	}
	return &padIds, rows.Err()
}

func (d PostgresDB) GetAuthors(
	ids []string,
) (*[]db.AuthorDB, error) {
	if len(ids) == 0 {
		return &[]db.AuthorDB{}, nil
	}

	ctx := context.Background()

	sqlStr, args, err := psql.
		Select(
			"ga.id",
			"ga.colorid",
			"ga.name",
			"ga.timestamp",
			"ga.token",
			"ga.created_at",
		).
		From("globalauthor ga").
		Where(sq.Eq{"ga.id": ids}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := d.pool.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	authorsMap := make(map[string]db.AuthorDB)

	for rows.Next() {
		author, err := ReadToAuthorDB(rows)
		if err != nil {
			return nil, err
		}

		authorsMap[author.ID] = *author
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	authors := make([]db.AuthorDB, 0, len(authorsMap))
	for _, a := range authorsMap {
		authors = append(authors, a)
	}

	return &authors, nil
}

func (d PostgresDB) SaveAuthor(author db.AuthorDB) error {
	if author.ID == "" {
		return errors.New("author ID is empty")
	}

	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO "globalauthor" (id, "colorid", name, timestamp, token, created_at) 
         VALUES ($1, $2, $3, $4, $5, NOW())
         ON CONFLICT (id) DO UPDATE SET
             "colorid" = EXCLUDED."colorid",
             name = EXCLUDED.name,
             timestamp = EXCLUDED.timestamp,
             updated_at = NOW(),
             token = COALESCE(EXCLUDED.token, "globalauthor".token)`,
		author.ID, author.ColorId, author.Name, author.Timestamp, author.Token)
	return err
}

func (d PostgresDB) GetAuthor(authorId string) (*db.AuthorDB, error) {
	ctx := context.Background()

	rows, err := d.pool.Query(ctx,
		`SELECT ga.id, ga."colorid", ga.name, ga.timestamp, ga.token, ga.created_at
         FROM "globalauthor" ga
         WHERE ga.id = $1`,
		authorId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authorDB db.AuthorDB

	for rows.Next() {
		author, err := ReadToAuthorDB(rows)
		if err != nil {
			return nil, err
		}
		authorDB = *author
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if authorDB.ID == "" {
		return nil, errors.New(AuthorNotFoundError)
	}

	return &authorDB, nil
}

func (d PostgresDB) SetAuthorByToken(token, authorId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`UPDATE "globalauthor" SET token = $1 WHERE id = $2`,
		token, authorId)
	if err != nil {
		return err
	}
	// If author doesn't exist yet, create with token
	_, err = d.pool.Exec(ctx,
		`INSERT INTO "globalauthor" (id, token, "colorid", timestamp, created_at)
         VALUES ($1, $2, '', 0, NOW())
         ON CONFLICT (id) DO UPDATE SET token = EXCLUDED.token`,
		authorId, token)
	return err
}

func (d PostgresDB) GetAuthorByToken(token string) (*string, error) {
	ctx := context.Background()

	var authorID string
	err := d.pool.QueryRow(ctx,
		`SELECT id FROM "globalauthor" WHERE token = $1`,
		token).Scan(&authorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(AuthorNotFoundError)
		}
		return nil, err
	}
	return &authorID, nil
}

func (d PostgresDB) SaveAuthorName(authorId string, authorName string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}
	ctx := context.Background()
	result, err := d.pool.Exec(ctx,
		`UPDATE "globalauthor" SET name = $1 WHERE id = $2`,
		authorName, authorId)
	if err != nil {
		return err
	}

	rs := result.RowsAffected()
	if rs == 0 {
		return errors.New(AuthorNotFoundError)
	}
	return err
}

func (d PostgresDB) SaveAuthorColor(authorId string, authorColor string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}
	ctx := context.Background()
	res, err := d.pool.Exec(ctx,
		`UPDATE "globalauthor" SET "colorid" = $1 WHERE id = $2`,
		authorColor, authorId)

	if err != nil {
		return err
	}
	rowsAffected := res.RowsAffected()
	if rowsAffected == 0 {
		return errors.New(AuthorNotFoundError)
	}

	return nil
}

// ============== REVISION METHODS ==============

func (d PostgresDB) SaveRevision(
	padId string,
	rev int,
	changeset string,
	text db.AText,
	pool db.RevPool,
	authorId *string,
	timestamp int64,
) error {
	exists, err := d.DoesPadExist(padId)
	if err != nil {
		return err
	}
	if !*exists {
		return errors.New(PadDoesNotExistError)
	}

	serializedPool, err := json.Marshal(pool)
	if err != nil {
		return fmt.Errorf("error serializing pool: %w", err)
	}

	ctx := context.Background()
	_, err = d.pool.Exec(ctx,
		`INSERT INTO "padrev" 
             (id, rev, changeset, "atexttext", "atextattribs", "authorid", timestamp, pool, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
         ON CONFLICT (id, rev) DO NOTHING`,
		padId, rev, changeset, text.Text, text.Attribs,
		authorId, timestamp, string(serializedPool))
	return err
}

func (d PostgresDB) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	ctx := context.Background()

	var revision db.PadSingleRevision
	var serializedPool string

	err := d.pool.QueryRow(ctx,
		`SELECT id, rev, changeset, "atexttext", "atextattribs", "authorid", 
                timestamp, pool 
         FROM "padrev" WHERE id = $1 AND rev = $2`,
		padId, rev).Scan(
		&revision.PadId, &revision.RevNum, &revision.Changeset,
		&revision.AText.Text, &revision.AText.Attribs,
		&revision.AuthorId, &revision.Timestamp, &serializedPool,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(PadRevisionNotFoundError)
		}
		return nil, fmt.Errorf("error scanning revision: %w", err)
	}

	if err := json.Unmarshal([]byte(serializedPool), &revision.Pool); err != nil {
		return nil, fmt.Errorf("error deserializing pool: %w", err)
	}

	return &revision, nil
}

func (d PostgresDB) GetRevisions(
	padId string,
	startRev int,
	endRev int,
) (*[]db.PadSingleRevision, error) {
	ctx := context.Background()

	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	rows, err := d.pool.Query(ctx,
		`SELECT id, rev, changeset, "atexttext", "atextattribs", "authorid", 
                timestamp, pool 
         FROM "padrev" 
         WHERE id = $1 AND rev >= $2 AND rev <= $3 
         ORDER BY rev ASC`,
		padId, startRev, endRev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []db.PadSingleRevision
	for rows.Next() {
		var rev db.PadSingleRevision
		var serializedPool string

		if err := rows.Scan(
			&rev.PadId, &rev.RevNum, &rev.Changeset,
			&rev.AText.Text, &rev.AText.Attribs,
			&rev.AuthorId, &rev.Timestamp, &serializedPool,
		); err != nil {
			return nil, fmt.Errorf("error scanning revision: %w", err)
		}
		if err := json.Unmarshal([]byte(serializedPool), &rev.Pool); err != nil {
			return nil, fmt.Errorf("error deserializing pool: %w", err)
		}
		revisions = append(revisions, rev)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(revisions) != (endRev - startRev + 1) {
		return nil, errors.New(PadRevisionNotFoundError)
	}

	return &revisions, nil
}

func (d PostgresDB) RemoveRevisionsOfPad(padId string) error {
	existingPad, err := d.DoesPadExist(padId)
	if err != nil {
		return err
	}
	if !*existingPad {
		return errors.New(PadDoesNotExistError)
	}

	ctx := context.Background()
	_, err = d.pool.Exec(ctx, `DELETE FROM "padrev" WHERE id = $1`, padId)
	return err
}

// ============== CHAT METHODS ==============

func (d PostgresDB) SaveChatMessage(
	padId string,
	head int,
	authorId *string,
	timestamp int64,
	text string,
) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO "padchat" ("padid", "padhead", "chattext", "authorid", timestamp, created_at)
         VALUES ($1, $2, $3, $4, $5, NOW())
         ON CONFLICT ("padid", "padhead") DO NOTHING`,
		padId, head, text, authorId, timestamp)
	return err
}

func (d PostgresDB) GetChatsOfPad(
	padId string,
	start int,
	end int,
) (*[]db.ChatMessageDBWithDisplayName, error) {
	ctx := context.Background()

	rows, err := d.pool.Query(ctx,
		`SELECT pc."padid", pc."padhead", pc."chattext", 
                pc."authorid", pc.timestamp, ga.name
         FROM "padchat" pc
         JOIN "globalauthor" ga ON ga.id = pc."authorid"
         WHERE pc."padid" = $1 AND pc."padhead" >= $2 AND pc."padhead" <= $3
         ORDER BY pc."padhead" ASC`,
		padId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chatMessages []db.ChatMessageDBWithDisplayName
	for rows.Next() {
		var msg db.ChatMessageDBWithDisplayName
		if err := rows.Scan(
			&msg.PadId, &msg.Head, &msg.Message,
			&msg.AuthorId, &msg.Time, &msg.DisplayName,
		); err != nil {
			return nil, err
		}
		chatMessages = append(chatMessages, msg)
	}

	return &chatMessages, rows.Err()
}

func (d PostgresDB) GetAuthorIdsOfPadChats(id string) (*[]string, error) {
	ctx := context.Background()

	rows, err := d.pool.Query(ctx,
		`SELECT DISTINCT "authorid" FROM "padchat" WHERE "padid" = $1`,
		id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authorIds []string
	for rows.Next() {
		var authorId string
		if err := rows.Scan(&authorId); err != nil {
			return nil, err
		}
		authorIds = append(authorIds, authorId)
	}
	return &authorIds, rows.Err()
}

func (d PostgresDB) RemoveChat(padId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx, `DELETE FROM "padchat" WHERE "padid" = $1`, padId)
	return err
}

// ============== GROUP METHODS ==============

func (d PostgresDB) SaveGroup(groupId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO "grouppadgroup" (id, created_at) VALUES ($1, NOW()) ON CONFLICT (id) DO NOTHING`,
		groupId)
	return err
}

func (d PostgresDB) RemoveGroup(groupId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx, `DELETE FROM "grouppadgroup" WHERE id = $1`, groupId)
	return err
}

func (d PostgresDB) GetGroup(groupId string) (*string, error) {
	ctx := context.Background()
	var foundGroup string
	err := d.pool.QueryRow(ctx,
		`SELECT id FROM "grouppadgroup" WHERE id = $1`,
		groupId).Scan(&foundGroup)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("group not found")
		}
		return nil, err
	}
	return &foundGroup, nil
}

// ============== SESSION METHODS ==============

func (d PostgresDB) GetSessionById(sessionID string) (*session2.Session, error) {
	ctx := context.Background()
	var s session2.Session
	err := d.pool.QueryRow(ctx,
		`SELECT id, "originalmaxage", expires, secure, "httponly", path, 
                "samesite", connections 
         FROM sessionstorage WHERE id = $1`,
		sessionID).Scan(
		&s.Id, &s.OriginalMaxAge, &s.Expires, &s.Secure,
		&s.HttpOnly, &s.Path, &s.SameSite, &s.Connections,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (d PostgresDB) SetSessionById(sessionID string, session session2.Session) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO sessionstorage 
             (id, "originalmaxage", expires, secure, "httponly", path, "samesite", connections, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
         ON CONFLICT (id) DO UPDATE SET
             "originalmaxage" = EXCLUDED."originalmaxage",
             expires = EXCLUDED.expires,
             secure = EXCLUDED.secure,
             "httponly" = EXCLUDED."httponly",
             path = EXCLUDED.path,
             "samesite" = EXCLUDED."samesite",
             connections = EXCLUDED.connections,
             updated_at = NOW()`,
		sessionID, session.OriginalMaxAge, session.Expires, session.Secure,
		session.HttpOnly, session.Path, session.SameSite, "")
	return err
}

func (d PostgresDB) RemoveSessionById(sid string) error {
	ctx := context.Background()
	result, err := d.pool.Exec(ctx, `DELETE FROM sessionstorage WHERE id = $1`, sid)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(SessionNotFoundError)
	}
	return nil
}

// ============== QUERY/SEARCH METHODS ==============

func (d PostgresDB) countQuery(pattern string) (*int, error) {
	ctx := context.Background()

	builder := psql.Select("COUNT(*)").From("pad")

	if pattern != "" {
		builder = builder.Where(sq.Like{"id": "%" + pattern + "%"})
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}

	var totalPads int
	err = d.pool.QueryRow(ctx, sql, args...).Scan(&totalPads)
	if err != nil {
		return nil, err
	}

	return &totalPads, nil
}

func (d PostgresDB) queryPad(
	pattern string,
	sortBy string,
	limit int,
	offset int,
	ascending bool,
) (*[]db.PadDBSearch, error) {
	ctx := context.Background()

	builder := psql.
		Select("id", "head", "updated_at").
		From("pad")

	if pattern != "" {
		builder = builder.Where(sq.Like{"id": "%" + pattern + "%"})
	}

	if sortBy == "padName" {
		if ascending {
			builder = builder.OrderBy("id ASC")
		} else {
			builder = builder.OrderBy("id DESC")
		}
	} else if sortBy == "lastEdited" {
		if ascending {
			builder = builder.OrderBy("updated_at ASC")
		} else {
			builder = builder.OrderBy("updated_at DESC")
		}
	}

	if limit > 0 {
		builder = builder.Limit(uint64(limit))
	}
	if offset > 0 {
		builder = builder.Offset(uint64(offset))
	}

	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := d.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var padSearch []db.PadDBSearch
	for rows.Next() {
		var padId string
		var head int
		var updatedAt time.Time
		if err := rows.Scan(&padId, &head, &updatedAt); err != nil {
			return nil, err
		}
		padSearch = append(padSearch, db.PadDBSearch{
			Padname:        padId,
			RevisionNumber: head,
			LastEdited:     updatedAt.UnixMilli(),
		})
	}

	return &padSearch, rows.Err()
}

func (d PostgresDB) QueryPad(
	offset int,
	limit int,
	sortBy string,
	ascending bool,
	pattern string,
) (*db.PadDBSearchResult, error) {
	padSearch, err := d.queryPad(pattern, sortBy, limit, offset, ascending)
	if err != nil {
		return nil, err
	}
	totalPads, err := d.countQuery(pattern)
	if err != nil {
		return nil, err
	}

	return &db.PadDBSearchResult{
		TotalPads: *totalPads,
		Pads:      *padSearch,
	}, nil
}

// ============== LIFECYCLE ==============

func (d PostgresDB) Close() error {
	d.pool.Close()
	return nil
}

type PostgresOptions struct {
	Username string
	Password string
	Port     int
	Host     string
	Database string
}

func NewPostgresDB(options PostgresOptions) (*PostgresDB, error) {
	ctx := context.Background()

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		options.Username, options.Password,
		options.Host, options.Port, options.Database,
	)

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	migrationManager := migrations.NewMigrationManager(sqlDB, migrations.DialectPostgres)
	if err := migrationManager.Run(); err != nil {
		sqlDB.Close()
		pool.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	sqlDB.Close()

	return &PostgresDB{
		options: options,
		pool:    pool,
	}, nil
}

var _ DataStore = (*PostgresDB)(nil)

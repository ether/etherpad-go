package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

func (d PostgresDB) GetAuthorIdsOfPadChats(id string) (*[]string, error) {
	var authorIds []string
	var resultedSQL, args, err = psql.
		Select("DISTINCT authorId").
		From("padChat").
		Where(sq.Eq{"padId": id}).
		ToSql()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()

	query, err := d.pool.Query(ctx, resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()
	for query.Next() {
		var authorId string
		query.Scan(&authorId)
		authorIds = append(authorIds, authorId)
	}
	return &authorIds, nil
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func (d PostgresDB) SaveGroup(groupId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO "grouppadgroup" (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`,
		groupId)
	return err
}

func (d PostgresDB) RemoveGroup(groupId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`DELETE FROM "grouppadgroup" WHERE id = $1`,
		groupId)
	return err
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

func (d PostgresDB) countQuery(pattern string) (*int, error) {
	ctx := context.Background()

	var countSQL string
	var args []interface{}

	if pattern != "" {
		countSQL = `
			SELECT COUNT(*) FROM pad 
			JOIN "padrev" ON "padrev".id = pad.id 
			WHERE "padrev".rev = (SELECT MAX(rev) FROM "padrev" WHERE "padrev".id = pad.id)
			AND pad.id LIKE $1`
		args = []interface{}{"%" + pattern + "%"}
	} else {
		countSQL = `
			SELECT COUNT(*) FROM pad 
			JOIN "padrev" ON "padrev".id = pad.id 
			WHERE "padrev".rev = (SELECT MAX(rev) FROM "padrev" WHERE "padrev".id = pad.id)`
	}

	var totalPads int
	err := d.pool.QueryRow(ctx, countSQL, args...).Scan(&totalPads)
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

	subQuery := psql.Select("MAX(rev)").
		From(`"padrev"`).
		Where(sq.Expr(`"padrev".id = pad.id`))

	subSQL, subArgs, err := subQuery.ToSql()
	if err != nil {
		return nil, err
	}

	builder := psql.
		Select("pad.id", "pad.data", `"padrev".timestamp`).
		From("pad").
		Join(`"padrev" ON "padrev".id = pad.id`).
		Where(sq.Expr(`"padrev".rev = (`+subSQL+`)`, subArgs...))

	if pattern != "" {
		builder = builder.Where(sq.Like{"pad.id": "%" + pattern + "%"})
	}

	if sortBy == "padName" {
		if ascending {
			builder = builder.OrderBy("pad.id ASC")
		} else {
			builder = builder.OrderBy("pad.id DESC")
		}
	}
	if limit > 0 {
		builder = builder.Limit(uint64(limit))
	}
	if offset > 0 {
		builder = builder.Offset(uint64(offset))
	}

	resultedSQL, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := d.pool.Query(ctx, resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var padSearch []db.PadDBSearch
	for rows.Next() {
		var padId, data string
		var timestamp int64
		if err := rows.Scan(&padId, &data, &timestamp); err != nil {
			return nil, err
		}
		var padDB db.PadDB
		if err := json.Unmarshal([]byte(data), &padDB); err != nil {
			return nil, err
		}
		padSearch = append(padSearch, db.PadDBSearch{
			Padname:        padId,
			RevisionNumber: padDB.RevNum,
			LastEdited:     timestamp,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &padSearch, nil
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

func (d PostgresDB) GetChatsOfPad(
	padId string,
	start int,
	end int,
) (*[]db.ChatMessageDBWithDisplayName, error) {
	ctx := context.Background()

	rows, err := d.pool.Query(ctx,
		`SELECT "padchat"."padid", "padchat"."padhead", "padchat"."chattext", 
		        "padchat"."authorid", "padchat".timestamp, "globalauthor".name
		 FROM "padchat"
		 JOIN "globalauthor" ON "globalauthor".id = "padchat"."authorid"
		 WHERE "padid" = $1 AND "padhead" >= $2 AND "padhead" <= $3
		 ORDER BY "padhead" ASC`,
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &chatMessages, nil
}

func (d PostgresDB) SaveChatHeadOfPad(padId string, head int) error {
	resultingPad, err := d.GetPad(padId)
	if err != nil {
		return err
	}
	resultingPad.ChatHead = head
	return d.CreatePad(padId, *resultingPad)
}

func (d PostgresDB) SaveChatMessage(
	padId string,
	head int,
	authorId *string,
	timestamp int64,
	text string,
) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO "padchat" ("padid", "padhead", "chattext", "authorid", timestamp)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT ("padid", "padhead") DO UPDATE SET
		     "chattext" = EXCLUDED."chattext",
		     "authorid" = EXCLUDED."authorid",
		     timestamp = EXCLUDED.timestamp`,
		padId, head, text, authorId, timestamp)
	return err
}

func (d PostgresDB) RemovePad(padID string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx, `DELETE FROM pad WHERE id = $1`, padID)
	return err
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

func (d PostgresDB) RemoveChat(padId string) error {
	ctx := context.Background()

	_, err := d.pool.Exec(ctx, `DELETE FROM "padchat" WHERE "padid" = $1`, padId)
	return err
}

func (d PostgresDB) RemovePad2ReadOnly(id string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx, `DELETE FROM pad2readonly WHERE id = $1`, id)
	return err
}

func (d PostgresDB) RemoveReadOnly2Pad(id string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx, `DELETE FROM readonly2pad WHERE id = $1`, id)
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
		     (id, "originalmaxage", expires, secure, "httponly", path, "samesite", connections)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id) DO UPDATE SET
		     "originalmaxage" = EXCLUDED."originalmaxage",
		     expires = EXCLUDED.expires,
		     secure = EXCLUDED.secure,
		     "httponly" = EXCLUDED."httponly",
		     path = EXCLUDED.path,
		     "samesite" = EXCLUDED."samesite",
		     connections = EXCLUDED.connections`,
		sessionID, session.OriginalMaxAge, session.Expires, session.Secure,
		session.HttpOnly, session.Path, session.SameSite, "")
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

func (d PostgresDB) RemoveSessionById(sid string) error {
	ctx := context.Background()
	result, err := d.pool.Exec(ctx,
		`DELETE FROM sessionstorage WHERE id = $1`, sid)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(SessionNotFoundError)
	}
	return nil
}

func (d PostgresDB) CreatePad(padID string, padDB db.PadDB) error {
	ctx := context.Background()

	marshalled, err := json.Marshal(padDB)
	if err != nil {
		return err
	}

	_, err = d.pool.Exec(ctx,
		`INSERT INTO pad (id, data) VALUES ($1, $2)
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data`,
		padID, string(marshalled))
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &padIds, nil
}

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
		     (id, rev, changeset, "atexttext", "atextattribs", "authorid", timestamp, pool)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id, rev) DO UPDATE SET
		     changeset = EXCLUDED.changeset,
		     "atexttext" = EXCLUDED."atexttext",
		     "atextattribs" = EXCLUDED."atextattribs",
		     "authorid" = EXCLUDED."authorid",
		     timestamp = EXCLUDED.timestamp,
		     pool = EXCLUDED.pool`,
		padId, rev, changeset, text.Text, text.Attribs,
		authorId, timestamp, string(serializedPool))
	return err
}

func (d PostgresDB) GetPad(padID string) (*db.PadDB, error) {
	ctx := context.Background()

	var data string
	err := d.pool.QueryRow(ctx,
		`SELECT data FROM pad WHERE id = $1`,
		padID).Scan(&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(PadDoesNotExistError)
		}
		return nil, err
	}

	var padDB db.PadDB
	if err := json.Unmarshal([]byte(data), &padDB); err != nil {
		return nil, err
	}

	return &padDB, nil
}

func (d PostgresDB) GetReadonlyPad(padId string) (*string, error) {
	ctx := context.Background()

	var readonlyId string
	err := d.pool.QueryRow(ctx,
		`SELECT data FROM pad2readonly WHERE id = $1`,
		padId).Scan(&readonlyId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(PadReadOnlyIdNotFoundError)
		}
		return nil, err
	}
	return &readonlyId, nil
}

func (d PostgresDB) CreatePad2ReadOnly(padId string, readonlyId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO pad2readonly (id, data) VALUES ($1, $2)
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data`,
		padId, readonlyId)
	return err
}

func (d PostgresDB) CreateReadOnly2Pad(padId string, readonlyId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO readonly2pad (id, data) VALUES ($1, $2)
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data`,
		readonlyId, padId)
	return err
}

func (d PostgresDB) GetReadOnly2Pad(id string) (*string, error) {
	ctx := context.Background()

	var padId string
	err := d.pool.QueryRow(ctx,
		`SELECT data FROM readonly2pad WHERE id = $1`,
		id).Scan(&padId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &padId, nil
}

func (d PostgresDB) SetAuthorByToken(token, authorId string) error {
	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO token2author (token, author) VALUES ($1, $2)
		 ON CONFLICT (token) DO UPDATE SET author = EXCLUDED.author`,
		token, authorId)
	return err
}

func (d PostgresDB) GetAuthor(author string) (*db.AuthorDB, error) {
	ctx := context.Background()

	rows, err := d.pool.Query(ctx,
		`SELECT "globalauthor".id, "globalauthor"."colorid", "globalauthor".name, 
		        "globalauthor".timestamp, "padrev".id
		 FROM "globalauthor"
		 LEFT JOIN "padrev" ON "globalauthor".id = "padrev"."authorid"
		 WHERE "globalauthor".id = $1`,
		author)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authorDB *db.AuthorDB

	for rows.Next() {
		var padID *string

		if authorDB == nil {
			authorDB = &db.AuthorDB{
				PadIDs: make(map[string]struct{}),
			}
			if err := rows.Scan(
				&authorDB.ID, &authorDB.ColorId, &authorDB.Name,
				&authorDB.Timestamp, &padID,
			); err != nil {
				return nil, err
			}
		} else {
			var dummy1, dummy2, dummy3, dummy4 interface{}
			if err := rows.Scan(&dummy1, &dummy2, &dummy3, &dummy4, &padID); err != nil {
				return nil, err
			}
		}

		if padID != nil {
			authorDB.PadIDs[*padID] = struct{}{}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if authorDB == nil {
		return nil, errors.New(AuthorNotFoundError)
	}

	return authorDB, nil
}

func (d PostgresDB) GetAuthorByToken(token string) (*string, error) {
	ctx := context.Background()

	var authorID string
	err := d.pool.QueryRow(ctx,
		`SELECT author FROM token2author WHERE token = $1`,
		token).Scan(&authorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(AuthorNotFoundError)
		}
		return nil, err
	}
	return &authorID, nil
}

func (d PostgresDB) SaveAuthor(author db.AuthorDB) error {
	if author.ID == "" {
		return errors.New("author ID is empty")
	}

	ctx := context.Background()
	_, err := d.pool.Exec(ctx,
		`INSERT INTO "globalauthor" (id, "colorid", name, timestamp) 
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET
		     "colorid" = EXCLUDED."colorid",
		     name = EXCLUDED.name,
		     timestamp = EXCLUDED.timestamp`,
		author.ID, author.ColorId, author.Name, author.Timestamp)
	return err
}

func (d PostgresDB) SaveAuthorName(authorId string, authorName string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	authorDB, err := d.GetAuthor(authorId)
	if err != nil {
		return err
	}

	authorDB.Name = &authorName
	return d.SaveAuthor(*authorDB)
}

func (d PostgresDB) SaveAuthorColor(authorId string, authorColor string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	authorDB, err := d.GetAuthor(authorId)
	if err != nil {
		return err
	}

	authorDB.ColorId = authorColor
	return d.SaveAuthor(*authorDB)
}

func (d PostgresDB) GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	ctx := context.Background()
	var meta db.PadMetaData
	var serializedPool string

	err = d.pool.QueryRow(ctx,
		`SELECT id, rev, changeset, "atexttext", "atextattribs", "authorid", 
		        timestamp, pool
		 FROM "padrev" WHERE id = $1 AND rev = $2`,
		padId, revNum).Scan(
		&meta.Id, &meta.RevNum, &meta.ChangeSet,
		&meta.Atext.Text, &meta.AtextAttribs,
		&meta.AuthorId, &meta.Timestamp, &serializedPool,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(PadRevisionNotFoundError)
		}
		return nil, err
	}

	if err := json.Unmarshal([]byte(serializedPool), &meta.PadPool); err != nil {
		return nil, err
	}

	return &meta, nil
}

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

	// Verify connection
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

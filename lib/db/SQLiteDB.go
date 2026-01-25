package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/db/migrations"
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
	_ "modernc.org/sqlite"
)

type SQLiteDB struct {
	path  string
	sqlDB *sql.DB
}

// ============== PAD METHODS ==============

func (d SQLiteDB) CreatePad(padID string, padDB db.PadDB) error {
	savedRevisions, err := json.Marshal(padDB.SavedRevisions)
	if err != nil {
		return fmt.Errorf("error marshaling saved revisions: %w", err)
	}

	pool, err := json.Marshal(padDB.Pool)
	if err != nil {
		return fmt.Errorf("error marshaling pool: %w", err)
	}

	resultedSQL, args, err := sq.
		Insert("pad").
		Columns("id", "head", "saved_revisions", "readonly_id", "pool", "chat_head",
			"public_status", "atext_text", "atext_attribs").
		Values(padID, padDB.Head, string(savedRevisions), padDB.ReadOnlyId, string(pool),
			padDB.ChatHead, padDB.PublicStatus, padDB.ATextText, padDB.ATextAttribs).
		Suffix(`ON CONFLICT(id) DO UPDATE SET
			head = excluded.head,
			saved_revisions = excluded.saved_revisions,
			readonly_id = excluded.readonly_id,
			pool = excluded.pool,
			chat_head = excluded.chat_head,
			public_status = excluded.public_status,
			atext_text = excluded.atext_text,
			atext_attribs = excluded.atext_attribs,
			updated_at = CURRENT_TIMESTAMP`).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) GetPad(padID string) (*db.PadDB, error) {
	resultedSQL, args, err := sq.
		Select("id", "head", "saved_revisions", "readonly_id", "pool", "chat_head",
			"public_status", "atext_text", "atext_attribs", "created_at", "updated_at").
		From("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(resultedSQL, args...)

	var padDB db.PadDB
	var savedRevisions, pool sql.NullString
	var readonlyId sql.NullString
	var createdAt, updatedAt sql.NullTime

	err = row.Scan(
		&padDB.ID, &padDB.Head, &savedRevisions, &readonlyId, &pool,
		&padDB.ChatHead, &padDB.PublicStatus, &padDB.ATextText, &padDB.ATextAttribs,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(PadDoesNotExistError)
		}
		return nil, err
	}

	if readonlyId.Valid {
		padDB.ReadOnlyId = &readonlyId.String
	}

	if savedRevisions.Valid {
		if err := json.Unmarshal([]byte(savedRevisions.String), &padDB.SavedRevisions); err != nil {
			return nil, fmt.Errorf("error unmarshaling saved revisions: %w", err)
		}
	}

	if pool.Valid {
		if err := json.Unmarshal([]byte(pool.String), &padDB.Pool); err != nil {
			return nil, fmt.Errorf("error unmarshaling pool: %w", err)
		}
	}

	if createdAt.Valid {
		padDB.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		padDB.UpdatedAt = &updatedAt.Time
	}

	return &padDB, nil
}

func (d SQLiteDB) DoesPadExist(padID string) (*bool, error) {
	resultedSQL, args, err := sq.
		Select("1").
		From("pad").
		Where(sq.Eq{"id": padID}).
		Limit(1).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(resultedSQL, args...)
	var exists int
	err = row.Scan(&exists)

	if errors.Is(err, sql.ErrNoRows) {
		falseVal := false
		return &falseVal, nil
	}
	if err != nil {
		return nil, err
	}

	trueVal := true
	return &trueVal, nil
}

func (d SQLiteDB) RemovePad(padID string) error {
	resultedSQL, args, err := sq.
		Delete("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) GetPadIds() (*[]string, error) {
	resultedSQL, _, err := sq.
		Select("id").
		From("pad").
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	var padIds []string
	for query.Next() {
		var padId string
		if err := query.Scan(&padId); err != nil {
			return nil, err
		}
		padIds = append(padIds, strings.TrimPrefix(padId, "pad:"))
	}

	return &padIds, query.Err()
}

func (d SQLiteDB) SaveChatHeadOfPad(padId string, head int) error {
	resultedSQL, args, err := sq.
		Update("pad").
		Set("chat_head", head).
		Set("updated_at", sq.Expr("CURRENT_TIMESTAMP")).
		Where(sq.Eq{"id": padId}).
		ToSql()

	if err != nil {
		return err
	}

	result, err := d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New(PadDoesNotExistError)
	}
	return nil
}

// ============== READONLY METHODS (simplified) ==============

func (d SQLiteDB) GetReadonlyPad(padId string) (*string, error) {
	resultedSQL, args, err := sq.
		Select("readonly_id").
		From("pad").
		Where(sq.Eq{"id": padId}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(resultedSQL, args...)
	var readonlyId sql.NullString
	err = row.Scan(&readonlyId)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New(PadDoesNotExistError)
	}
	if err != nil {
		return nil, err
	}

	if !readonlyId.Valid {
		return nil, errors.New(PadReadOnlyIdNotFoundError)
	}

	return &readonlyId.String, nil
}

func (d SQLiteDB) SetReadOnlyId(padId string, readonlyId string) error {
	resultedSQL, args, err := sq.
		Update("pad").
		Set("readonly_id", readonlyId).
		Set("updated_at", sq.Expr("CURRENT_TIMESTAMP")).
		Where(sq.Eq{"id": padId}).
		ToSql()

	if err != nil {
		return err
	}

	result, err := d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New(PadDoesNotExistError)
	}
	return nil
}

func (d SQLiteDB) GetPadByReadOnlyId(readonlyId string) (*string, error) {
	resultedSQL, args, err := sq.
		Select("id").
		From("pad").
		Where(sq.Eq{"readonly_id": readonlyId}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(resultedSQL, args...)
	var padId string
	err = row.Scan(&padId)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &padId, nil
}

// ============== AUTHOR METHODS ==============

func (d SQLiteDB) GetPadIdsOfAuthor(authorId string) (*[]string, error) {
	resultedSQL, args, err := sq.
		Select("id").
		From("padRev").
		Where(sq.Eq{"authorId": authorId}).
		ToSql()

	if err != nil {
		return nil, err
	}
	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()
	var padIds []string
	for query.Next() {
		var padId string
		if err := query.Scan(&padId); err != nil {
			return nil, err
		}
	}
	return &padIds, query.Err()
}

func (d SQLiteDB) GetAuthors(
	ids []string,
) (*[]db.AuthorDB, error) {
	if len(ids) == 0 {
		return &[]db.AuthorDB{}, nil
	}

	sqlStr, args, err := sq.
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

	rows, err := d.sqlDB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []db.AuthorDB

	for rows.Next() {
		foundAuthor, err := ReadToAuthorDB(rows)
		if err != nil {
			return nil, err
		}

		authors = append(authors, *foundAuthor)
	}

	return &authors, rows.Err()
}

func (d SQLiteDB) SaveAuthor(author db.AuthorDB) error {
	if author.ID == "" {
		return errors.New("author ID is empty")
	}

	resultedSQL, args, err := sq.
		Insert("globalAuthor").
		Columns("id", "colorId", "name", "timestamp", "token").
		Values(author.ID, author.ColorId, author.Name, author.Timestamp, author.Token).
		Suffix(`ON CONFLICT(id) DO UPDATE SET 
			colorId = excluded.colorId, 
			name = excluded.name, 
			timestamp = excluded.timestamp,
			token = COALESCE(excluded.token, globalAuthor.token)`).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) GetAuthor(authorId string) (*db.AuthorDB, error) {
	resultedSQL, args, err := sq.
		Select("ga.id", "ga.colorId", "ga.name", "ga.timestamp", "ga.token", "ga.created_at").
		From("globalAuthor ga").
		Where(sq.Eq{"ga.id": authorId}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	var authorDB *db.AuthorDB

	for query.Next() {
		foundAuthor, err := ReadToAuthorDB(query)
		if err != nil {
			return nil, err
		}
		authorDB = foundAuthor
	}

	if err := query.Err(); err != nil {
		return nil, err
	}

	if authorDB == nil {
		return nil, errors.New(AuthorNotFoundError)
	}

	return authorDB, nil
}

func (d SQLiteDB) SetAuthorByToken(token, authorId string) error {
	// First try to update existing author
	updateSQL, updateArgs, err := sq.
		Update("globalAuthor").
		Set("token", token).
		Where(sq.Eq{"id": authorId}).
		ToSql()

	if err != nil {
		return err
	}

	result, err := d.sqlDB.Exec(updateSQL, updateArgs...)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		return nil
	}

	// If author doesn't exist, create with token
	insertSQL, insertArgs, err := sq.
		Insert("globalAuthor").
		Columns("id", "token", "colorId", "timestamp").
		Values(authorId, token, "", 0).
		Suffix("ON CONFLICT(id) DO UPDATE SET token = excluded.token").
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(insertSQL, insertArgs...)
	return err
}

func (d SQLiteDB) GetAuthorByToken(token string) (*string, error) {
	resultedSQL, args, err := sq.
		Select("id").
		From("globalAuthor").
		Where(sq.Eq{"token": token}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(resultedSQL, args...)
	var authorID string
	err = row.Scan(&authorID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New(AuthorNotFoundError)
	}
	if err != nil {
		return nil, err
	}

	return &authorID, nil
}

func (d SQLiteDB) SaveAuthorName(authorId string, authorName string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	resultedSQL, args, err := sq.
		Update("globalAuthor").
		Set("name", authorName).
		Where(sq.Eq{"id": authorId}).
		ToSql()

	if err != nil {
		return err
	}

	result, err := d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		return err
	}
	rs, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rs == 0 {
		return errors.New(AuthorNotFoundError)
	}

	return err
}

func (d SQLiteDB) SaveAuthorColor(authorId string, authorColor string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	resultedSQL, args, err := sq.
		Update("globalAuthor").
		Set("colorId", authorColor).
		Where(sq.Eq{"id": authorId}).
		ToSql()

	if err != nil {
		return err
	}

	result, err := d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		return err
	}
	rs, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rs == 0 {
		return errors.New(AuthorNotFoundError)
	}
	return err
}

// ============== REVISION METHODS ==============

func (d SQLiteDB) SaveRevision(
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

	marshalled, err := json.Marshal(pool)
	if err != nil {
		return fmt.Errorf("failed to marshal pool: %w", err)
	}

	// Use INSERT OR IGNORE for write-once semantics
	toSql, args, err := sq.Insert("padRev").
		Columns("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp", "pool").
		Values(padId, rev, changeset, text.Text, text.Attribs, authorId, timestamp, string(marshalled)).
		Suffix("ON CONFLICT(id, rev) DO NOTHING"). // Write-once: ignore duplicates
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(toSql, args...)
	return err
}

func (d SQLiteDB) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	retrievedSql, args, err := sq.
		Select("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp", "pool").
		From("padRev").
		Where(sq.Eq{"id": padId}).
		Where(sq.Eq{"rev": rev}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(retrievedSql, args...)

	var revisionDB db.PadSingleRevision
	var serializedPool string

	err = row.Scan(
		&revisionDB.PadId, &revisionDB.RevNum, &revisionDB.Changeset,
		&revisionDB.AText.Text, &revisionDB.AText.Attribs,
		&revisionDB.AuthorId, &revisionDB.Timestamp, &serializedPool,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New(PadRevisionNotFoundError)
	}
	if err != nil {
		return nil, fmt.Errorf("error scanning revision: %w", err)
	}

	if err := json.Unmarshal([]byte(serializedPool), &revisionDB.Pool); err != nil {
		return nil, fmt.Errorf("error deserializing pool: %w", err)
	}

	return &revisionDB, nil
}

func (d SQLiteDB) GetRevisions(padId string, startRev int, endRev int) (*[]db.PadSingleRevision, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := sq.
		Select("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp", "pool").
		From("padRev").
		Where(sq.Eq{"id": padId}).
		Where(sq.GtOrEq{"rev": startRev}).
		Where(sq.LtOrEq{"rev": endRev}).
		OrderBy("rev ASC").
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	var revisions []db.PadSingleRevision
	for query.Next() {
		var serializedPool string
		var revisionDB db.PadSingleRevision

		if err := query.Scan(
			&revisionDB.PadId, &revisionDB.RevNum, &revisionDB.Changeset,
			&revisionDB.AText.Text, &revisionDB.AText.Attribs,
			&revisionDB.AuthorId, &revisionDB.Timestamp, &serializedPool,
		); err != nil {
			return nil, fmt.Errorf("error scanning revision: %w", err)
		}

		if err := json.Unmarshal([]byte(serializedPool), &revisionDB.Pool); err != nil {
			return nil, fmt.Errorf("error deserializing pool: %w", err)
		}
		revisions = append(revisions, revisionDB)
	}

	if err := query.Err(); err != nil {
		return nil, err
	}

	if len(revisions) != (endRev - startRev + 1) {
		return nil, errors.New(PadRevisionNotFoundError)
	}

	return &revisions, nil
}

func (d SQLiteDB) RemoveRevisionsOfPad(padId string) error {
	existingPad, err := d.DoesPadExist(padId)
	if err != nil {
		return err
	}
	if !*existingPad {
		return errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := sq.
		Delete("padRev").
		Where(sq.Eq{"id": padId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

// ============== CHAT METHODS ==============

func (d SQLiteDB) SaveChatMessage(
	padId string,
	head int,
	authorId *string,
	timestamp int64,
	text string,
) error {
	// Write-once: use ON CONFLICT DO NOTHING
	resultedSQL, args, err := sq.
		Insert("padChat").
		Columns("padId", "padHead", "chatText", "authorId", "created_at").
		Values(padId, head, text, authorId, timestamp).
		Suffix("ON CONFLICT(padId, padHead) DO NOTHING"). // Write-once
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) GetChatsOfPad(
	padId string,
	start int,
	end int,
) (*[]db.ChatMessageDBWithDisplayName, error) {
	resultedSQL, args, err := sq.
		Select("pc.padId", "pc.padHead", "pc.chatText", "pc.authorId", "pc.created_at", "ga.name").
		From("padChat pc").
		Join("globalAuthor ga ON ga.id = pc.authorId").
		Where(sq.Eq{"pc.padId": padId}).
		Where(sq.GtOrEq{"pc.padHead": start}).
		Where(sq.LtOrEq{"pc.padHead": end}).
		OrderBy("pc.padHead ASC").
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	var chatMessages []db.ChatMessageDBWithDisplayName
	for query.Next() {
		var chatMessage db.ChatMessageDBWithDisplayName
		if err := query.Scan(
			&chatMessage.PadId, &chatMessage.Head, &chatMessage.Message,
			&chatMessage.AuthorId, &chatMessage.Time, &chatMessage.DisplayName,
		); err != nil {
			return nil, err
		}
		chatMessages = append(chatMessages, chatMessage)
	}

	return &chatMessages, query.Err()
}

func (d SQLiteDB) GetAuthorIdsOfPadChats(id string) (*[]string, error) {
	resultedSQL, args, err := sq.
		Select("DISTINCT authorId").
		From("padChat").
		Where(sq.Eq{"padId": id}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	var authorIds []string
	for query.Next() {
		var authorId string
		if err := query.Scan(&authorId); err != nil {
			return nil, err
		}
		authorIds = append(authorIds, authorId)
	}

	return &authorIds, query.Err()
}

func (d SQLiteDB) RemoveChat(padId string) error {
	resultedSQL, args, err := sq.
		Delete("padChat").
		Where(sq.Eq{"padId": padId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

// ============== GROUP METHODS ==============

func (d SQLiteDB) SaveGroup(groupId string) error {
	resultedSQL, args, err := sq.Insert("groupPadGroup").
		Columns("id").
		Values(groupId).
		Suffix("ON CONFLICT(id) DO NOTHING").
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) RemoveGroup(groupId string) error {
	resultedSQL, args, err := sq.Delete("groupPadGroup").
		Where(sq.Eq{"id": groupId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) GetGroup(groupId string) (*string, error) {
	resultedSQL, args, err := sq.
		Select("id").
		From("groupPadGroup").
		Where(sq.Eq{"id": groupId}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(resultedSQL, args...)
	var foundGroup string
	err = row.Scan(&foundGroup)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("group not found")
	}
	if err != nil {
		return nil, err
	}

	return &foundGroup, nil
}

// ============== SESSION METHODS ==============

func (d SQLiteDB) GetSessionById(sessionID string) (*session2.Session, error) {
	createdSQL, arr, err := sq.
		Select("id", "originalMaxAge", "expires", "secure", "httpOnly", "path", "sameSite", "connections").
		From("sessionstorage").
		Where(sq.Eq{"id": sessionID}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(createdSQL, arr...)

	var possibleSession session2.Session
	err = row.Scan(
		&possibleSession.Id, &possibleSession.OriginalMaxAge, &possibleSession.Expires,
		&possibleSession.Secure, &possibleSession.HttpOnly, &possibleSession.Path,
		&possibleSession.SameSite, &possibleSession.Connections,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &possibleSession, nil
}

func (d SQLiteDB) SetSessionById(sessionID string, session session2.Session) error {
	retrievedSql, inserts, err := sq.Insert("sessionstorage").
		Columns("id", "originalMaxAge", "expires", "secure", "httpOnly", "path", "sameSite", "connections").
		Values(sessionID, session.OriginalMaxAge, session.Expires, session.Secure,
			session.HttpOnly, session.Path, session.SameSite, "").
		Suffix(`ON CONFLICT(id) DO UPDATE SET 
			originalMaxAge = excluded.originalMaxAge, 
			expires = excluded.expires, 
			secure = excluded.secure, 
			httpOnly = excluded.httpOnly, 
			path = excluded.path, 
			sameSite = excluded.sameSite, 
			connections = excluded.connections`).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(retrievedSql, inserts...)
	return err
}

func (d SQLiteDB) RemoveSessionById(sid string) error {
	resultedSQL, args, err := sq.Delete("sessionstorage").Where(sq.Eq{"id": sid}).ToSql()
	if err != nil {
		return err
	}

	result, err := d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New(SessionNotFoundError)
	}

	return nil
}

// ============== QUERY/SEARCH METHODS ==============

func (d SQLiteDB) countQuery(pattern string) (*int, error) {
	builder := sq.Select("COUNT(*)").From("pad")

	if pattern != "" {
		builder = builder.Where(sq.Like{"id": "%" + pattern + "%"})
	}

	countSQL, countArgs, err := builder.ToSql()
	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(countSQL, countArgs...)
	var totalPads int
	if err := row.Scan(&totalPads); err != nil {
		return nil, err
	}

	return &totalPads, nil
}

func (d SQLiteDB) queryPad(
	pattern string,
	sortBy string,
	limit int,
	offset int,
	ascending bool,
) (*[]db.PadDBSearch, error) {
	builder := sq.
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

	resultedSQL, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	var padSearch []db.PadDBSearch
	for query.Next() {
		var padId string
		var head int
		var updatedAt time.Time

		if err := query.Scan(&padId, &head, &updatedAt); err != nil {
			return nil, err
		}

		padSearch = append(padSearch, db.PadDBSearch{
			Padname:        padId,
			RevisionNumber: head,
			LastEdited:     updatedAt.UnixMilli(),
		})
	}

	return &padSearch, query.Err()
}

func (d SQLiteDB) QueryPad(
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

func (d SQLiteDB) GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := sq.
		Select("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp", "pool").
		From("padRev").
		Where(sq.Eq{"id": padId}).
		Where(sq.Eq{"rev": revNum}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(resultedSQL, args...)

	var padMetaData db.PadMetaData
	var serializedPool string

	err = row.Scan(
		&padMetaData.Id, &padMetaData.RevNum, &padMetaData.ChangeSet,
		&padMetaData.Atext.Text, &padMetaData.AtextAttribs,
		&padMetaData.AuthorId, &padMetaData.Timestamp, &serializedPool,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New(PadRevisionNotFoundError)
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(serializedPool), &padMetaData.PadPool); err != nil {
		return nil, err
	}

	return &padMetaData, nil
}

// ============== LIFECYCLE ==============

func (d SQLiteDB) Close() error {
	return d.sqlDB.Close()
}

// NewSQLiteDB creates a new SQLiteDB and returns a pointer to it.
func NewSQLiteDB(path string) (*SQLiteDB, error) {
	if path == ":memory" {
		path = "file::memory:?cache=shared"
	}

	sqlDb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if strings.Contains(path, ":memory:") {
		sqlDb.SetMaxOpenConns(1)
	}

	if _, err = sqlDb.Exec("PRAGMA journal_mode = WAL"); err != nil {
		sqlDb.Close()
		return nil, err
	}
	if _, err = sqlDb.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		sqlDb.Close()
		return nil, err
	}
	if _, err = sqlDb.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDb.Close()
		return nil, err
	}

	migrationManager := migrations.NewMigrationManager(sqlDb, migrations.DialectSQLite)
	if err := migrationManager.Run(); err != nil {
		sqlDb.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &SQLiteDB{
		path:  path,
		sqlDB: sqlDb,
	}, nil
}

var _ DataStore = (*SQLiteDB)(nil)

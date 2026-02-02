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
	mysql2 "github.com/go-sql-driver/mysql"
)

type MysqlDB struct {
	options MySQLOptions
	sqlDB   *sql.DB
}

func (d MysqlDB) Ping() error {
	return d.sqlDB.Ping()
}

func (d MysqlDB) GetPadIdsOfAuthor(authorId string) (*[]string, error) {
	resultedSQL, args, err := mysql.
		Select("DISTINCT pr.id").
		From("padRev pr").
		Where(sq.Eq{"pr.authorId": authorId}).
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
		padIds = append(padIds, padId)
	}
	return &padIds, query.Err()
}

var mysql = sq.StatementBuilder.PlaceholderFormat(sq.Question)

// ============== PAD METHODS ==============

func (d MysqlDB) CreatePad(padID string, padDB db.PadDB) error {
	savedRevisions, err := json.Marshal(padDB.SavedRevisions)
	if err != nil {
		return fmt.Errorf("error marshaling saved revisions: %w", err)
	}

	pool, err := json.Marshal(padDB.Pool)
	if err != nil {
		return fmt.Errorf("error marshaling pool: %w", err)
	}

	resultedSQL, args, err := mysql.
		Insert("pad").
		Columns("id", "head", "saved_revisions", "readonly_id", "pool", "chat_head",
			"public_status", "atext_text", "atext_attribs").
		Values(padID, padDB.Head, string(savedRevisions), padDB.ReadOnlyId, string(pool),
			padDB.ChatHead, padDB.PublicStatus, padDB.ATextText, padDB.ATextAttribs).
		Suffix(`ON DUPLICATE KEY UPDATE
			head = VALUES(head),
			saved_revisions = VALUES(saved_revisions),
			readonly_id = VALUES(readonly_id),
			pool = VALUES(pool),
			chat_head = VALUES(chat_head),
			public_status = VALUES(public_status),
			atext_text = VALUES(atext_text),
			atext_attribs = VALUES(atext_attribs)`).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) GetPad(padID string) (*db.PadDB, error) {
	resultedSQL, args, err := mysql.
		Select("id", "head", "saved_revisions", "readonly_id", "pool", "chat_head",
			"public_status", "atext_text", "atext_attribs", "created_at", "updated_at").
		From("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return nil, err
	}

	row := d.sqlDB.QueryRow(resultedSQL, args...)

	padDb, err := ReadToPadDB(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(PadDoesNotExistError)
		}
		return nil, err
	}

	return padDb, nil
}

func (d MysqlDB) DoesPadExist(padID string) (*bool, error) {
	resultedSQL, args, err := mysql.
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

func (d MysqlDB) RemovePad(padID string) error {
	resultedSQL, args, err := mysql.
		Delete("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) GetPadIds() (*[]string, error) {
	resultedSQL, _, err := mysql.
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

func (d MysqlDB) SaveChatHeadOfPad(padId string, head int) error {
	resultedSQL, args, err := mysql.
		Update("pad").
		Set("chat_head", head).
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

func (d MysqlDB) GetReadonlyPad(padId string) (*string, error) {
	resultedSQL, args, err := mysql.
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

func (d MysqlDB) SetReadOnlyId(padId string, readonlyId string) error {
	resultedSQL, args, err := mysql.
		Update("pad").
		Set("readonly_id", readonlyId).
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

func (d MysqlDB) GetPadByReadOnlyId(readonlyId string) (*string, error) {
	resultedSQL, args, err := mysql.
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

func (d MysqlDB) SaveAuthor(author db.AuthorDB) error {
	if author.ID == "" {
		return errors.New("author ID is empty")
	}

	resultedSQL, args, err := mysql.
		Insert("globalAuthor").
		Columns("id", "colorId", "name", "timestamp", "token").
		Values(author.ID, author.ColorId, author.Name, author.Timestamp, author.Token).
		Suffix(`ON DUPLICATE KEY UPDATE 
			colorId = VALUES(colorId), 
			name = VALUES(name), 
			timestamp = VALUES(timestamp),
			token = COALESCE(VALUES(token), token)`).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) GetAuthors(ids []string) (*[]db.AuthorDB, error) {
	if len(ids) == 0 {
		return &[]db.AuthorDB{}, nil
	}
	resultedSQL, args, err := mysql.
		Select("id", "colorId", "name", "timestamp", "token", "created_at").
		From("globalAuthor").
		Where(sq.Eq{"id": ids}).
		ToSql()
	if err != nil {
		return nil, err
	}
	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()
	var authors []db.AuthorDB
	for query.Next() {
		foundAuthor, err := ReadToAuthorDB(query)
		if err != nil {
			return nil, err
		}
		authors = append(authors, *foundAuthor)
	}
	return &authors, query.Err()
}

func (d MysqlDB) GetAuthor(authorId string) (*db.AuthorDB, error) {
	resultedSQL, args, err := mysql.
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

func (d MysqlDB) SetAuthorByToken(token, authorId string) error {
	// First try to update existing author
	updateSQL, updateArgs, err := mysql.
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
	insertSQL, insertArgs, err := mysql.
		Insert("globalAuthor").
		Columns("id", "token", "colorId", "timestamp").
		Values(authorId, token, "", 0).
		Suffix("ON DUPLICATE KEY UPDATE token = VALUES(token)").
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(insertSQL, insertArgs...)
	return err
}

func (d MysqlDB) GetAuthorByToken(token string) (*string, error) {
	resultedSQL, args, err := mysql.
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

func (d MysqlDB) SaveAuthorName(authorId string, authorName string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	resultedSQL, args, err := mysql.
		Update("globalAuthor").
		Set("name", authorName).
		Where(sq.Eq{"id": authorId}).
		ToSql()

	if err != nil {
		return err
	}

	rs, err := d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}
	rowsAffected, err := rs.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New(AuthorNotFoundError)
	}

	return err
}

func (d MysqlDB) SaveAuthorColor(authorId string, authorColor string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	resultedSQL, args, err := mysql.
		Update("globalAuthor").
		Set("colorId", authorColor).
		Where(sq.Eq{"id": authorId}).
		ToSql()

	if err != nil {
		return err
	}

	res, err := d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New(AuthorNotFoundError)
	}
	return err
}

// ============== REVISION METHODS ==============

func (d MysqlDB) SaveRevision(
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

	// Use INSERT IGNORE for write-once semantics
	toSql, args, err := mysql.Insert("padRev").
		Columns("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp", "pool").
		Values(padId, rev, changeset, text.Text, text.Attribs, authorId, timestamp, string(marshalled)).
		Suffix("ON DUPLICATE KEY UPDATE id = id"). // No-op update for write-once
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(toSql, args...)
	return err
}

func (d MysqlDB) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	retrievedSql, args, err := mysql.
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

func (d MysqlDB) GetRevisions(padId string, startRev int, endRev int) (*[]db.PadSingleRevision, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := mysql.
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

func (d MysqlDB) RemoveRevisionsOfPad(padId string) error {
	existingPad, err := d.DoesPadExist(padId)
	if err != nil {
		return err
	}
	if !*existingPad {
		return errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := mysql.
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

func (d MysqlDB) SaveChatMessage(
	padId string,
	head int,
	authorId *string,
	timestamp int64,
	text string,
) error {
	// Write-once: use INSERT IGNORE or ON DUPLICATE KEY UPDATE with no-op
	resultedSQL, args, err := mysql.
		Insert("padChat").
		Columns("padId", "padHead", "chatText", "authorId", "timestamp").
		Values(padId, head, text, authorId, timestamp).
		Suffix("ON DUPLICATE KEY UPDATE padId = padId"). // No-op for write-once
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) GetChatsOfPad(
	padId string,
	start int,
	end int,
) (*[]db.ChatMessageDBWithDisplayName, error) {
	resultedSQL, args, err := mysql.
		Select("pc.padId", "pc.padHead", "pc.chatText", "pc.authorId", "pc.timestamp", "ga.name").
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

func (d MysqlDB) GetAuthorIdsOfPadChats(id string) (*[]string, error) {
	resultedSQL, args, err := mysql.
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

func (d MysqlDB) RemoveChat(padId string) error {
	resultedSQL, args, err := mysql.
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

func (d MysqlDB) SaveGroup(groupId string) error {
	resultedSQL, args, err := mysql.Insert("groupPadGroup").
		Columns("id").
		Values(groupId).
		Suffix("ON DUPLICATE KEY UPDATE id = id").
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) RemoveGroup(groupId string) error {
	resultedSQL, args, err := mysql.Delete("groupPadGroup").
		Where(sq.Eq{"id": groupId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) GetGroup(groupId string) (*string, error) {
	resultedSQL, args, err := mysql.
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

func (d MysqlDB) GetSessionById(sessionID string) (*session2.Session, error) {
	createdSQL, arr, err := mysql.
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

func (d MysqlDB) SetSessionById(sessionID string, session session2.Session) error {
	retrievedSql, inserts, err := mysql.Insert("sessionstorage").
		Columns("id", "originalMaxAge", "expires", "secure", "httpOnly", "path", "sameSite", "connections").
		Values(sessionID, session.OriginalMaxAge, session.Expires, session.Secure,
			session.HttpOnly, session.Path, session.SameSite, "").
		Suffix(`ON DUPLICATE KEY UPDATE 
			originalMaxAge = VALUES(originalMaxAge), 
			expires = VALUES(expires), 
			secure = VALUES(secure), 
			httpOnly = VALUES(httpOnly), 
			path = VALUES(path), 
			sameSite = VALUES(sameSite), 
			connections = VALUES(connections)`).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(retrievedSql, inserts...)
	return err
}

func (d MysqlDB) RemoveSessionById(sid string) error {
	resultedSQL, args, err := mysql.Delete("sessionstorage").Where(sq.Eq{"id": sid}).ToSql()
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

func (d MysqlDB) countQuery(pattern string) (*int, error) {
	builder := mysql.Select("COUNT(*)").From("pad")

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

func (d MysqlDB) queryPad(
	pattern string,
	sortBy string,
	limit int,
	offset int,
	ascending bool,
) (*[]db.PadDBSearch, error) {
	builder := mysql.
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

func (d MysqlDB) QueryPad(
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

func (d MysqlDB) GetServerVersion() (string, error) {
	var version string
	err := d.sqlDB.QueryRow("SELECT version FROM server_version ORDER BY updated_at DESC LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return version, err
}

func (d MysqlDB) SaveServerVersion(version string) error {
	_, err := d.sqlDB.Exec("INSERT INTO server_version (version, updated_at) VALUES (?, NOW(6)) ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at)", version)
	return err
}

// ============== LIFECYCLE ==============

func (d MysqlDB) Close() error {
	return d.sqlDB.Close()
}

type MySQLOptions struct {
	Username string
	Password string
	Port     int
	Host     string
	Database string
}

func NewMySQLDB(options MySQLOptions) (*MysqlDB, error) {
	mySQLConf := mysql2.NewConfig()
	mySQLConf.User = options.Username
	mySQLConf.Passwd = options.Password
	mySQLConf.Net = "tcp"
	mySQLConf.Addr = fmt.Sprintf("%s:%d", options.Host, options.Port)
	mySQLConf.DBName = options.Database
	mySQLConf.ParseTime = true
	mySQLConf.Loc = nil

	sqlDb, err := sql.Open("mysql", mySQLConf.FormatDSN())
	if err != nil {
		return nil, err
	}

	sqlDb.SetMaxOpenConns(25)
	sqlDb.SetMaxIdleConns(5)

	migrationManager := migrations.NewMigrationManager(sqlDb, migrations.DialectMySQL)
	if err := migrationManager.Run(); err != nil {
		sqlDb.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &MysqlDB{
		options: options,
		sqlDB:   sqlDb,
	}, nil
}

var _ DataStore = (*MysqlDB)(nil)

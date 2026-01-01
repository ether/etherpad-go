package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/db/migrations"
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
	"github.com/ory/fosite"
	_ "modernc.org/sqlite"
)

type SQLiteDB struct {
	path  string
	sqlDB *sql.DB
}

func (d SQLiteDB) SaveGroup(groupId string) error {
	var resultedSQL, args, err = sq.Insert("groupPadGroup").
		Columns("id").
		Values(groupId).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}

	return nil
}

func (d SQLiteDB) RemoveGroup(groupId string) error {
	var resultedSQL, args, err = sq.
		Delete("groupPadGroup").
		Where(sq.Eq{"id": groupId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}

	return nil
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
		Select("*").
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
		var revisionDB db.PadSingleRevision
		var serializedPool string
		query.Scan(&revisionDB.PadId, &revisionDB.RevNum, &revisionDB.Changeset, &revisionDB.AText.Text, &revisionDB.AText.Attribs, &revisionDB.AuthorId, &revisionDB.Timestamp, &serializedPool)
		revisions = append(revisions, revisionDB)
	}

	if len(revisions) != (endRev - startRev + 1) {
		return nil, errors.New(PadRevisionNotFoundError)
	}

	return &revisions, nil
}

func (d SQLiteDB) countQuery(pattern string) (*int, error) {
	subQuery := sq.Select("MAX(rev)").
		From("padRev").
		Where(sq.Eq{"padRev.id": sq.Expr("pad.id")})

	subSQL, subArgs, err := subQuery.ToSql()
	if err != nil {
		return nil, err
	}

	var countBuilder = sq.
		Select("COUNT(*)").
		From("pad").
		Join("padRev ON padRev.id = pad.id").
		Where(sq.Expr("padRev.rev = ("+subSQL+")", subArgs...))

	if pattern != "" {
		countBuilder = countBuilder.Where(sq.Like{"pad.id": "%" + pattern + "%"})
	}

	countSQL, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	countQuery, err := d.sqlDB.Query(countSQL, countArgs...)
	if err != nil {
		return nil, err
	}
	defer countQuery.Close()

	var totalPads int
	for countQuery.Next() {
		countQuery.Scan(&totalPads)
	}

	return &totalPads, nil
}

func (d SQLiteDB) queryPad(pattern string, sortBy string, limit int, offset int, ascending bool) (*[]db.PadDBSearch, error) {
	subQuery := sq.Select("MAX(rev)").
		From("padRev").
		Where(sq.Eq{"padRev.id": sq.Expr("pad.id")})

	subSQL, subArgs, err := subQuery.ToSql()
	if err != nil {
		return nil, err
	}

	var builder = sq.
		Select("pad.id", "pad.data", "padRev.timestamp").
		From("pad").
		Join("padRev ON padRev.id = pad.id").
		Where(sq.Expr("padRev.rev = ("+subSQL+")", subArgs...))

	if pattern != "" {
		builder = builder.Where(sq.Like{"pad.id": "%" + pattern + "%"})
	}

	if sortBy == "padName" {
		if !ascending {
			builder = builder.OrderBy("pad.id DESC")
		} else {
			builder = builder.OrderBy("pad.id ASC")
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
		var data string
		var timestamp int64
		query.Scan(&padId, &data, &timestamp)
		var padDB db.PadDB
		err = json.Unmarshal([]byte(data), &padDB)
		if err != nil {
			return nil, err
		}
		padSearch = append(padSearch, db.PadDBSearch{
			Padname:        padId,
			RevisionNumber: padDB.RevNum,
			LastEdited:     timestamp,
		})
	}
	return &padSearch, nil
}

func (d SQLiteDB) QueryPad(offset int, limit int, sortBy string, ascending bool, pattern string) (*db.PadDBSearchResult, error) {

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

func (d SQLiteDB) GetChatsOfPad(padId string, start int, end int) (*[]db.ChatMessageDBWithDisplayName, error) {
	var resultedSQL, args, err = sq.
		Select("padChat.padid, padChat.padHead, padChat.chatText, padChat.authorId, padChat.timestamp, globalAuthor.name").
		From("padChat").
		Join("globalAuthor ON globalAuthor.id = padChat.authorId").
		Where(sq.Eq{"padId": padId}).
		Where(sq.GtOrEq{"padHead": start}).
		Where(sq.LtOrEq{"padHead": end}).
		OrderBy("padHead ASC").
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
		query.Scan(&chatMessage.PadId, &chatMessage.Head, &chatMessage.Message, &chatMessage.AuthorId, &chatMessage.Time, &chatMessage.DisplayName)
		chatMessages = append(chatMessages, chatMessage)
	}
	return &chatMessages, nil
}

func (d SQLiteDB) SaveChatHeadOfPad(padId string, head int) error {
	var resultingPad, err = d.GetPad(padId)
	if err != nil {
		return err
	}
	resultingPad.ChatHead = head
	d.CreatePad(padId, *resultingPad)
	return nil
}

func (d SQLiteDB) SaveChatMessage(padId string, head int, authorId *string, timestamp int64, text string) error {
	var resultedSQL, args, err = sq.
		Insert("padChat").
		Columns("padId", "padHead", "chatText", "authorId", "timestamp").
		Values(padId, head, text, authorId, timestamp).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	return err
}

func (d SQLiteDB) RemovePad(padID string) error {
	var resultedSQL, args, err = sq.
		Delete("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
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

func (d SQLiteDB) RemoveChat(padId string) error {
	var resultedSQL, args, err = sq.
		Delete("padChat").
		Where(sq.Eq{"padId": padId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) RemovePad2ReadOnly(id string) error {
	var resultedSQL, args, err = sq.
		Delete("pad2readonly").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) RemoveReadOnly2Pad(id string) error {
	var resultedSQL, args, err = sq.
		Delete("readonly2pad").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d SQLiteDB) GetGroup(groupId string) (*string, error) {
	var resultedSQL, args, err = sq.
		Select("id").
		From("groupPadGroup").
		Where(sq.Eq{"id": groupId}).
		ToSql()

	if err != nil {
		return nil, err
	}
	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()
	var foundGroup string
	for query.Next() {
		query.Scan(&foundGroup)
		return &foundGroup, nil
	}
	return nil, errors.New("group not found")
}

func (d SQLiteDB) GetSessionById(sessionID string) (*session2.Session, error) {
	var createdSQL, arr, err = sq.Select("*").From("sessionstorage").Where(sq.Eq{"id": sessionID}).ToSql()
	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(createdSQL, arr...)

	if err != nil {
		return nil, err
	}
	defer query.Close()

	for query.Next() {
		var possibleSession session2.Session
		query.Scan(&possibleSession.Id, &possibleSession.OriginalMaxAge, &possibleSession.Expires, &possibleSession.Secure, &possibleSession.HttpOnly, &possibleSession.Path, &possibleSession.SameSite, &possibleSession.Connections)
		return &possibleSession, nil
	}

	return nil, nil
}

func (d SQLiteDB) SetSessionById(sessionID string, session session2.Session) error {
	var retrievedSql, inserts, _ = sq.Insert("sessionstorage").Columns("id", "originalMaxAge", "expires", "secure", "httpOnly", "path", "sameSite", "connections").
		Values(sessionID, session.OriginalMaxAge, session.Expires, session.Secure, session.HttpOnly, session.Path, session.SameSite, "").ToSql()

	_, err := d.sqlDB.Exec(retrievedSql, inserts...)

	if err != nil {
		return err
	}
	return nil
}

func (d SQLiteDB) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	var retrievedSql, args, _ = sq.Select("*").From("padRev").Where(sq.Eq{"id": padId}).Where(sq.Eq{"rev": rev}).ToSql()

	query, err := d.sqlDB.Query(retrievedSql, args...)
	if err != nil {
		return nil, err
	}

	defer query.Close()

	for query.Next() {
		var revisionDB db.PadSingleRevision
		var serializedPool string
		query.Scan(&revisionDB.PadId, &revisionDB.RevNum, &revisionDB.Changeset, &revisionDB.AText.Text, &revisionDB.AText.Attribs, &revisionDB.AuthorId, &revisionDB.Timestamp, &serializedPool)
		if err := json.Unmarshal([]byte(serializedPool), &revisionDB.Pool); err != nil {
			return nil, fmt.Errorf("error deserializing pool: %v", err)
		}
		return &revisionDB, nil
	}

	return nil, errors.New(PadRevisionNotFoundError)
}

func (d SQLiteDB) DoesPadExist(padID string) (*bool, error) {
	var resultedSQL, args, err = sq.
		Select("id").
		From("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	for query.Next() {
		trueVal := true
		return &trueVal, nil
	}

	falseVal := false
	return &falseVal, nil
}

func (d SQLiteDB) RemoveSessionById(sid string) error {

	var foundSession, err = d.GetSessionById(sid)
	if err != nil {
		return err
	}

	if foundSession == nil {
		return errors.New(SessionNotFoundError)
	}

	resultedSQL, args, err := sq.Delete("sessionstorage").Where(sq.Eq{"id": sid}).ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}

	return nil
}

func (d SQLiteDB) CreatePad(padID string, padDB db.PadDB) error {

	_, notFound := d.GetPad(padID)

	var marshalled, err = json.Marshal(padDB)

	if err != nil {
		return err
	}

	var resultedSQL string
	var args []interface{}
	var err1 error

	if notFound != nil {
		resultedSQL, args, err1 = sq.
			Insert("pad").
			Columns("id", "data").
			Values(padID, string(marshalled)).ToSql()
	} else {
		resultedSQL, args, err1 = sq.
			Update("pad").
			Set("data", string(marshalled)).
			Where(sq.Eq{
				"id": padID,
			}).ToSql()
	}
	if err1 != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}

	return nil
}

func (d SQLiteDB) GetPadIds() (*[]string, error) {
	var padIds []string
	var resultedSQL, _, err = sq.
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

	for query.Next() {
		var padId string
		query.Scan(&padId)
		padIds = append(padIds, strings.TrimPrefix(padId, "pad:"))
	}

	return &padIds, nil
}

func (d SQLiteDB) SaveRevision(padId string, rev int, changeset string, text db.AText, pool db.RevPool, authorId *string, timestamp int64) error {
	exists, err := d.DoesPadExist(padId)
	if err != nil {
		return err
	}

	if !*exists {
		return errors.New(PadDoesNotExistError)
	}

	serializedPool, err := json.Marshal(pool)
	if err != nil {
		return fmt.Errorf("error serializing pool: %v", err)
	}

	toSql, i, err := sq.Insert("padRev").
		Columns("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp", "pool").
		Values(padId, rev, changeset, text.Text, text.Attribs, authorId, timestamp, serializedPool).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(toSql, i...)

	if err != nil {
		return err
	}

	return nil
}

func (d SQLiteDB) GetPad(padID string) (*db.PadDB, error) {
	var resultedSQL, args, err = sq.
		Select("data").
		From("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	var padDB *db.PadDB
	for query.Next() {
		var data string
		query.Scan(&data)
		err = json.Unmarshal([]byte(data), &padDB)
		if err != nil {
			return nil, err
		}
	}

	if padDB == nil {
		return nil, errors.New(PadDoesNotExistError)
	}

	return padDB, nil
}

func (d SQLiteDB) GetReadonlyPad(padId string) (*string, error) {

	resultedSQL, args, err := sq.
		Select("data").
		From("pad2readonly").
		Where(sq.Eq{"id": padId}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	for query.Next() {
		var readonlyId string
		query.Scan(&readonlyId)
		return &readonlyId, nil
	}

	return nil, errors.New(PadReadOnlyIdNotFoundError)
}

func (d SQLiteDB) CreatePad2ReadOnly(padId string, readonlyId string) error {
	var resultedSQL, args, err = sq.
		Insert("pad2readonly").
		Columns("id", "data").
		Values(padId, readonlyId).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d SQLiteDB) CreateReadOnly2Pad(padId string, readonlyId string) error {
	var resultedSQL, args, err = sq.
		Insert("readonly2pad").
		Columns("id", "data").
		Values(readonlyId, padId).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}
	return nil
}

func (d SQLiteDB) GetReadOnly2Pad(id string) (*string, error) {
	var resultedSQL, args, err = sq.
		Select("data").
		From("readonly2pad").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	for query.Next() {
		var padId string
		query.Scan(&padId)
		return &padId, nil
	}

	return nil, nil
}

func (d SQLiteDB) SetAuthorByToken(token, authorId string) error {
	var resulltedSQL, arg, _ = sq.
		Insert("token2author").
		Columns("token,author").
		Values(token, authorId).ToSql()

	_, err := d.sqlDB.Exec(resulltedSQL, arg...)

	if err != nil {
		return err
	}

	return nil
}

/**
 * Returns the Author Obj of the author
 * @param {String} author The id of the author
 */
func (d SQLiteDB) GetAuthor(author string) (*db.AuthorDB, error) {

	var resultedSQL, args, err = sq.Select("*").
		From("globalAuthor").
		Where(sq.Eq{"id": author}).ToSql()

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()
	for query.Next() {
		var authorDB *db.AuthorDB
		var authorCopy db.AuthorDB
		query.Scan(&authorCopy.ID, &authorCopy.ColorId, &authorCopy.Name, &authorCopy.Timestamp)
		authorDB = &authorCopy
		return authorDB, nil
	}

	return nil, errors.New(AuthorNotFoundError)
}

func (d SQLiteDB) GetAuthorByToken(token string) (*string, error) {
	var resultedSQL, args, err = sq.
		Select("author").
		From("token2author").
		Where(sq.Eq{"token": token}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()
	for query.Next() {
		var authorID string
		query.Scan(&authorID)
		return &authorID, nil
	}
	return nil, errors.New(AuthorNotFoundError)
}

func (d SQLiteDB) SaveAuthor(author db.AuthorDB) error {
	if author.ID == "" {
		return errors.New("author ID is empty")
	}
	var foundAuthor, err = d.GetAuthor(author.ID)
	if err != nil && err.Error() != AuthorNotFoundError {
		return err
	}

	if foundAuthor == nil {
		var resultedSQL, i, err = sq.
			Insert("globalAuthor").
			Columns("id", "colorId", "name", "timestamp").
			Values(author.ID, author.ColorId, author.Name, author.Timestamp).
			ToSql()
		_, err = d.sqlDB.Exec(resultedSQL, i...)
		if err != nil {
			return err
		}
	} else {
		var resultedSQL, i, err = sq.
			Update("globalAuthor").
			Set("colorId", author.ColorId).
			Set("name", author.Name).
			Set("timestamp", author.Timestamp).
			Where(sq.Eq{"id": author.ID}).
			ToSql()
		_, err = d.sqlDB.Exec(resultedSQL, i...)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d SQLiteDB) SaveAuthorName(authorId string, authorName string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}
	var authorString, err = d.GetAuthor(authorId)

	if err != nil || authorString == nil {
		return err
	}

	authorString.Name = &authorName
	d.SaveAuthor(*authorString)
	return nil
}

func (d SQLiteDB) SaveAuthorColor(authorId string, authorColor string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	var authorString, err = d.GetAuthor(authorId)

	if err != nil || authorString == nil {
		return errors.New("author not found")
	}

	authorString.ColorId = authorColor
	d.SaveAuthor(*authorString)
	return nil
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
		Select("*").
		From("padRev").
		Where(sq.Eq{"id": padId}).
		Where(sq.Eq{"rev": revNum}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	for query.Next() {
		var padMetaData db.PadMetaData
		var serializedPool string
		err := query.Scan(&padMetaData.Id, &padMetaData.RevNum, &padMetaData.ChangeSet, &padMetaData.Atext.Text, &padMetaData.AtextAttribs, &padMetaData.AuthorId, &padMetaData.Timestamp, &serializedPool)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(serializedPool), &padMetaData.PadPool); err != nil {
			return nil, err
		}

		return &padMetaData, nil
	}

	return nil, errors.New(PadRevisionNotFoundError)
}

func (d SQLiteDB) SaveAccessToken(token string, data fosite.Requester) error {
	var resultedSQL, args, err = sq.
		Insert("access_tokens").
		Columns("token", "data").
		Values(token, data).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d SQLiteDB) Close() error {
	return d.sqlDB.Close()
}

// NewSQLiteDB This function creates a new SQLiteDB and returns a pointer to it.
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
	if _, err = sqlDb.Exec("PRAGMA busy_timeout = 5000"); err != nil { // 5s Timeout
		sqlDb.Close()
		return nil, err
	}
	if _, err = sqlDb.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDb.Close()
		return nil, err
	}

	// Run migrations
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

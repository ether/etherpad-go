package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
	_ "github.com/lib/pq"
	"github.com/ory/fosite"
)

type PostgresDB struct {
	options PostgresOptions
	sqlDB   *sql.DB
}

func (d PostgresDB) SaveGroup(groupId string) error {
	var resultedSQL, args, err = psql.Insert("groupPadGroup").
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

func (d PostgresDB) RemoveGroup(groupId string) error {
	var resultedSQL, args, err = psql.Delete("groupPadGroup").
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

func (d PostgresDB) GetRevisions(padId string, startRev int, endRev int) (*[]db.PadSingleRevision, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := psql.
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
		query.Scan(&revisionDB.PadId, &revisionDB.RevNum, &revisionDB.Changeset, &revisionDB.AText.Text, &revisionDB.AText.Attribs, &revisionDB.AuthorId, &revisionDB.Timestamp)
		revisions = append(revisions, revisionDB)
	}

	if len(revisions) != (endRev - startRev + 1) {
		println("Revision is", len(revisions), endRev, startRev+1)
		return nil, errors.New(PadRevisionNotFoundError)
	}

	return &revisions, nil
}

func (d PostgresDB) countQuery(pattern string) (*int, error) {
	subQuery := psql.Select("MAX(rev)").
		From("padRev").
		Where(sq.Expr("padRev.id = pad.id"))

	subSQL, subArgs, err := subQuery.ToSql()
	if err != nil {
		return nil, err
	}

	var countBuilder = psql.
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

func (d PostgresDB) queryPad(pattern string, sortBy string, limit int, offset int, ascending bool) (*[]db.PadDBSearch, error) {
	subQuery := psql.Select("MAX(rev)").
		From("padRev").
		Where(sq.Expr("padRev.id = pad.id"))

	subSQL, subArgs, err := subQuery.ToSql()
	if err != nil {
		return nil, err
	}

	var builder = psql.
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

func (d PostgresDB) QueryPad(offset int, limit int, sortBy string, ascending bool, pattern string) (*db.PadDBSearchResult, error) {

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

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func (d PostgresDB) GetChatsOfPad(padId string, start int, end int) (*[]db.ChatMessageDBWithDisplayName, error) {
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) SaveChatHeadOfPad(padId string, head int) error {
	var resultingPad, err = d.GetPad(padId)
	if err != nil {
		return err
	}
	resultingPad.ChatHead = head
	d.CreatePad(padId, *resultingPad)
	return nil
}

func (d PostgresDB) SaveChatMessage(padId string, head int, authorId *string, timestamp int64, text string) error {
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) RemovePad(padID string) error {
	var resultedSQL, args, err = psql.
		Delete("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
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

	resultedSQL, args, err := psql.
		Delete("padRev").
		Where(sq.Eq{"id": padId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d PostgresDB) RemoveChat(padId string) error {
	var resultedSQL, args, err = psql.
		Delete("padChat").
		Where(sq.Eq{"padId": padId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d PostgresDB) RemovePad2ReadOnly(id string) error {
	var resultedSQL, args, err = psql.
		Delete("pad2readonly").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d PostgresDB) RemoveReadOnly2Pad(id string) error {
	var resultedSQL, args, err = psql.
		Delete("readonly2pad").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d PostgresDB) GetGroup(groupId string) (*string, error) {
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) GetSessionById(sessionID string) (*session2.Session, error) {
	var createdSQL, arr, err = psql.Select("*").From("sessionstorage").Where(sq.Eq{"id": sessionID}).ToSql()
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

func (d PostgresDB) SetSessionById(sessionID string, session session2.Session) error {
	var retrievedSql, inserts, _ = psql.Insert("sessionstorage").Columns("id", "originalMaxAge", "expires", "secure", "httpOnly", "path", "sameSite", "connections").
		Values(sessionID, session.OriginalMaxAge, session.Expires, session.Secure, session.HttpOnly, session.Path, session.SameSite, "").ToSql()

	_, err := d.sqlDB.Exec(retrievedSql, inserts...)

	if err != nil {
		return err
	}

	return nil
}

func (d PostgresDB) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	var retrievedSql, args, _ = psql.Select("*").From("padRev").Where(sq.Eq{"id": padId}).Where(sq.Eq{"rev": rev}).ToSql()

	query, err := d.sqlDB.Query(retrievedSql, args...)
	if err != nil {
		println("Error getting revision", err)
	}

	defer query.Close()

	for query.Next() {
		var revisionDB db.PadSingleRevision
		query.Scan(&revisionDB.PadId, &revisionDB.RevNum, &revisionDB.Changeset, &revisionDB.AText.Text, &revisionDB.AText.Attribs, &revisionDB.AuthorId, &revisionDB.Timestamp)
		return &revisionDB, nil
	}

	return nil, errors.New(PadRevisionNotFoundError)
}

func (d PostgresDB) DoesPadExist(padID string) (*bool, error) {
	var resultedSQL, args, err = psql.
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

	for query.Next() {
		trueVal := true
		return &trueVal, nil
	}

	defer query.Close()
	falseVal := false
	return &falseVal, nil
}

func (d PostgresDB) RemoveSessionById(sid string) error {

	var foundSession, err = d.GetSessionById(sid)
	if err != nil {
		return err
	}

	if foundSession == nil {
		return errors.New(SessionNotFoundError)
	}

	resultedSQL, args, err := psql.Delete("sessionstorage").Where(sq.Eq{"id": sid}).ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}

	return nil
}

func (d PostgresDB) CreatePad(padID string, padDB db.PadDB) error {

	_, notFound := d.GetPad(padID)

	var marshalled, err = json.Marshal(padDB)

	if err != nil {
		return err
	}

	var resultedSQL string
	var args []interface{}
	var err1 error

	if notFound != nil {
		resultedSQL, args, err1 = psql.
			Insert("pad").
			Columns("id", "data").
			Values(padID, string(marshalled)).ToSql()
	} else {
		resultedSQL, args, err1 = psql.
			Update("pad").
			Set("data", string(marshalled)).
			Where(sq.Eq{
				"id": padID,
			}).ToSql()
	}
	if err1 != nil {
		return err1
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}

	return err
}

func (d PostgresDB) GetPadIds() (*[]string, error) {
	var padIds []string
	var resultedSQL, _, err = psql.
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

func (d PostgresDB) SaveRevision(padId string, rev int, changeset string, text apool.AText, pool apool.APool, authorId *string, timestamp int64) error {
	exists, err := d.DoesPadExist(padId)
	if err != nil {
		return err
	}

	if !*exists {
		return errors.New(PadDoesNotExistError)
	}

	toSql, i, err := psql.Insert("padRev").
		Columns("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp").
		Values(padId, rev, changeset, text.Text, text.Attribs, authorId, timestamp).
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

func (d PostgresDB) GetPad(padID string) (*db.PadDB, error) {

	var resultedSQL, args, err = psql.
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

func (d PostgresDB) GetReadonlyPad(padId string) (*string, error) {
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) CreatePad2ReadOnly(padId string, readonlyId string) error {
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) CreateReadOnly2Pad(padId string, readonlyId string) error {
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) GetReadOnly2Pad(id string) (*string, error) {
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) SetAuthorByToken(token, authorId string) error {
	var resulltedSQL, arg, _ = psql.
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
func (d PostgresDB) GetAuthor(author string) (*db.AuthorDB, error) {

	var resultedSQL, args, err = psql.Select("*").
		From("globalAuthor").
		Where(sq.Eq{"id": author}).ToSql()

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	var authorDB *db.AuthorDB
	for query.Next() {
		var authorCopy db.AuthorDB
		query.Scan(&authorCopy.ID, &authorCopy.ColorId, &authorCopy.Name, &authorCopy.Timestamp)
		authorDB = &authorCopy
		return authorDB, nil
	}

	return nil, errors.New(AuthorNotFoundError)
}

func (d PostgresDB) GetAuthorByToken(token string) (*string, error) {
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) SaveAuthor(author db.AuthorDB) error {
	if author.ID == "" {
		return errors.New("author ID is empty")
	}
	var foundAuthor, err = d.GetAuthor(author.ID)
	if err != nil && err.Error() != AuthorNotFoundError {
		return err
	}

	if foundAuthor == nil {
		var resultedSQL, i, err = psql.
			Insert("globalAuthor").
			Columns("id", "colorId", "name", "timestamp").
			Values(author.ID, author.ColorId, author.Name, author.Timestamp).
			ToSql()
		_, err = d.sqlDB.Exec(resultedSQL, i...)
		if err != nil {
			return err
		}
	} else {
		var resultedSQL, i, err = psql.
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

func (d PostgresDB) SaveAuthorName(authorId string, authorName string) error {
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

func (d PostgresDB) SaveAuthorColor(authorId string, authorColor string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	var authorString, err = d.GetAuthor(authorId)

	if err != nil || authorString == nil {
		return err
	}

	authorString.ColorId = authorColor
	d.SaveAuthor(*authorString)
	return nil
}

func (d PostgresDB) GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := psql.
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

	var padMetaData db.PadMetaData
	for query.Next() {
		err := query.Scan(&padMetaData.Id, &padMetaData.RevNum, &padMetaData.ChangeSet, &padMetaData.Atext.Text, &padMetaData.AtextAttribs, &padMetaData.AuthorId, &padMetaData.Timestamp)
		if err != nil {
			return nil, err
		}
		return &padMetaData, nil
	}
	defer query.Close()

	return nil, errors.New(PadRevisionNotFoundError)
}

func (d PostgresDB) Close() error {
	return d.sqlDB.Close()
}

type PostgresOptions struct {
	Username string
	Password string
	Port     int
	Host     string
	Database string
}

// NewPostgresDB This function creates a new PostgresDB and returns a pointer to it.
func NewPostgresDB(options PostgresOptions) (*PostgresDB, error) {
	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", options.Username, options.Password, options.Host, options.Port, options.Database)
	sqlDb, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, err
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS pad (id TEXT PRIMARY KEY, data TEXT)")

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS globalAuthor(id TEXT PRIMARY KEY, colorId TEXT, name TEXT, timestamp BIGINT)")
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS padRev(id TEXT, rev INTEGER, changeset TEXT, atextText TEXT, atextAttribs TEXT, authorId TEXT, timestamp BIGINT, PRIMARY KEY (id, rev), FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS token2author(token TEXT PRIMARY KEY, author TEXT, FOREIGN KEY(author) REFERENCES globalAuthor(id) ON DELETE CASCADE)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS globalAuthorPads(id TEXT NOT NULL, padID TEXT NOT NULL,  PRIMARY KEY(id, padID), FOREIGN KEY(id) REFERENCES globalAuthor(id) ON DELETE CASCADE, FOREIGN KEY(padID) REFERENCES pad(id) ON DELETE CASCADE)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS pad2readonly(id TEXT PRIMARY KEY, data TEXT, FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)")
	if err != nil {
		return nil, err
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS readonly2pad(id TEXT PRIMARY KEY, data TEXT, FOREIGN KEY(data) REFERENCES pad(id) ON DELETE CASCADE)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS sessionstorage(id TEXT PRIMARY KEY, originalMaxAge INTEGER, expires TEXT, secure BOOLEAN, httpOnly BOOLEAN, path TEXT, sameSite TEXT, connections TEXT)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS groupPadGroup(id TEXT PRIMARY KEY, name TEXT)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS padChat(padId TEXT NOT NULL, padHead INTEGER,  chatText TEXT NOT NULL, authorId TEXT, timestamp BIGINT, PRIMARY KEY(padId, padHead), FOREIGN KEY(padId) REFERENCES pad(id) ON DELETE CASCADE)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS accesstoken(token TEXT PRIMARY KEY, data BYTEA)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS refreshtoken(token TEXT PRIMARY KEY, data BYTEA)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS accesstokentorequestid(requestID TEXT PRIMARY KEY, token TEXT)")

	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS refreshtokentorequestid(requestID TEXT PRIMARY KEY, token TEXT)")

	if err != nil {
		return nil, err
	}

	return &PostgresDB{
		options: options,
		sqlDB:   sqlDb,
	}, nil
}

func (d PostgresDB) SaveAccessToken(token string, data fosite.Requester) error {
	insertSQL, args, err := psql.
		Insert("accesstoken").
		Columns("token", "data").
		Values(token, data).
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(insertSQL, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d PostgresDB) GetAccessTokenRequestID(requestID string) (*string, error) {
	var query, args, err = psql.Select("token").
		From("accesstokentorequestid").
		Where(sq.Eq{"requestID": requestID}).
		ToSql()
	if err != nil {
		return nil, err
	}
	result, err := d.sqlDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	for result.Next() {
		var token string
		result.Scan(&token)
		return &token, nil
	}
	return nil, errors.New("access token request ID not found")
}

func (d PostgresDB) SaveAccessTokenRequestID(requestID string, token string) error {
	insertSQL, args, err := psql.
		Insert("accesstokentorequestid").
		Columns("requestID", "token").
		Values(requestID, token).
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(insertSQL, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d PostgresDB) GetAccessToken(signature string) (*fosite.Requester, error) {
	var query, args, err = psql.Select("data").
		From("accesstoken").
		Where(sq.Eq{"token": signature}).
		ToSql()
	if err != nil {
		return nil, err
	}
	result, err := d.sqlDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	for result.Next() {
		var dataBytes []byte
		result.Scan(&dataBytes)
		var requester fosite.Requester
		err = json.Unmarshal(dataBytes, &requester)
		if err != nil {
			return nil, err
		}
		return &requester, nil
	}
	return nil, errors.New("access token not found")
}

func (d PostgresDB) DeleteAccessToken(signature string) error {
	deleteSQL, args, err := psql.
		Delete("accesstoken").
		Where(sq.Eq{"token": signature}).ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(deleteSQL, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d PostgresDB) SaveRefreshToken(token string, data db.StoreRefreshToken) error {
	insertSQL, args, err := psql.
		Insert("refreshtoken").
		Columns("token", "data").
		Values(token, data).
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(insertSQL, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d PostgresDB) SaveRefreshTokenRequestID(requestID string, token string) error {
	insertSQL, args, err := psql.
		Insert("refreshtokentorequestid").
		Columns("requestID", "token").
		Values(requestID, token).
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(insertSQL, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d PostgresDB) GetRefreshToken(signature string) (*db.StoreRefreshToken, error) {
	var query, args, err = psql.Select("data").
		From("refreshtoken").
		Where(sq.Eq{"token": signature}).ToSql()
	if err != nil {
		return nil, err
	}
	result, err :=
		d.sqlDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	for result.Next() {
		var dataBytes []byte
		result.Scan(&dataBytes)
		var storeRefreshToken db.StoreRefreshToken
		err = json.Unmarshal(dataBytes, &storeRefreshToken)
		if err != nil {
			return nil, err
		}
		return &storeRefreshToken, nil
	}
	return nil, errors.New("refresh token not found")

}

func (d PostgresDB) DeleteRefreshToken(signature string) error {
	deleteSQL, args, err := psql.
		Delete("refreshtoken").
		Where(sq.Eq{"token": signature}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(deleteSQL, args...)
	if err != nil {
		return err
	}
	return nil
}

func (d PostgresDB) GetRefreshTokenRequestID(requestID string) (*string, error) {
	var query, args, err = psql.Select("token").
		From("refreshtokentorequestid").
		Where(sq.Eq{"requestID": requestID}).
		ToSql()
	if err != nil {
		return nil, err
	}
	result, err := d.sqlDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	for result.Next() {
		var token string
		result.Scan(&token)
		return &token, nil
	}
	return nil, errors.New("refresh token request ID not found")
}

var _ DataStore = (*PostgresDB)(nil)

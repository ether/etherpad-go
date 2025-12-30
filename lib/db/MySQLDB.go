package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
	mysql2 "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type MysqlDB struct {
	options MySQLOptions
	sqlDB   *sql.DB
}

func (d MysqlDB) SaveGroup(groupId string) error {
	var resultedSQL, args, err = mysql.Insert("groupPadGroup").
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

func (d MysqlDB) RemoveGroup(groupId string) error {
	var resultedSQL, args, err = mysql.Delete("groupPadGroup").
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

func (d MysqlDB) GetRevisions(padId string, startRev int, endRev int) (*[]db.PadSingleRevision, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := mysql.
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
		var serializedPool string
		var revisionDB db.PadSingleRevision
		query.Scan(&revisionDB.PadId, &revisionDB.RevNum, &revisionDB.Changeset, &revisionDB.AText.Text, &revisionDB.AText.Attribs, &revisionDB.AuthorId, &revisionDB.Timestamp, &serializedPool)
		if err := json.Unmarshal([]byte(serializedPool), &revisionDB.Pool); err != nil {
			return nil, fmt.Errorf("error deserializing pool: %v", err)
		}
		revisions = append(revisions, revisionDB)
	}

	if len(revisions) != (endRev - startRev + 1) {
		println("Revision is", len(revisions), endRev, startRev+1)
		return nil, errors.New(PadRevisionNotFoundError)
	}

	return &revisions, nil
}

func (d MysqlDB) countQuery(pattern string) (*int, error) {
	subQuery := mysql.Select("MAX(rev)").
		From("padRev").
		Where(sq.Expr("padRev.id = pad.id"))

	subSQL, subArgs, err := subQuery.ToSql()
	if err != nil {
		return nil, err
	}

	var countBuilder = mysql.
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

func (d MysqlDB) queryPad(pattern string, sortBy string, limit int, offset int, ascending bool) (*[]db.PadDBSearch, error) {
	subQuery := mysql.Select("MAX(rev)").
		From("padRev").
		Where(sq.Expr("padRev.id = pad.id"))

	subSQL, subArgs, err := subQuery.ToSql()
	if err != nil {
		return nil, err
	}

	var builder = mysql.
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

func (d MysqlDB) QueryPad(offset int, limit int, sortBy string, ascending bool, pattern string) (*db.PadDBSearchResult, error) {

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

var mysql = sq.StatementBuilder.PlaceholderFormat(sq.Question)

func (d MysqlDB) GetChatsOfPad(padId string, start int, end int) (*[]db.ChatMessageDBWithDisplayName, error) {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) SaveChatHeadOfPad(padId string, head int) error {
	var resultingPad, err = d.GetPad(padId)
	if err != nil {
		return err
	}
	resultingPad.ChatHead = head
	d.CreatePad(padId, *resultingPad)
	return nil
}

func (d MysqlDB) SaveChatMessage(padId string, head int, authorId *string, timestamp int64, text string) error {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) RemovePad(padID string) error {
	var resultedSQL, args, err = mysql.
		Delete("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
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

func (d MysqlDB) RemoveChat(padId string) error {
	var resultedSQL, args, err = mysql.
		Delete("padChat").
		Where(sq.Eq{"padId": padId}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) RemovePad2ReadOnly(id string) error {
	var resultedSQL, args, err = mysql.
		Delete("pad2readonly").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) RemoveReadOnly2Pad(id string) error {
	var resultedSQL, args, err = mysql.
		Delete("readonly2pad").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(resultedSQL, args...)
	return err
}

func (d MysqlDB) GetGroup(groupId string) (*string, error) {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) GetSessionById(sessionID string) (*session2.Session, error) {
	var createdSQL, arr, err = mysql.Select("*").From("sessionstorage").Where(sq.Eq{"id": sessionID}).ToSql()
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

func (d MysqlDB) SetSessionById(sessionID string, session session2.Session) error {
	var retrievedSql, inserts, _ = mysql.Insert("sessionstorage").Columns("id", "originalMaxAge", "expires", "secure", "httpOnly", "path", "sameSite", "connections").
		Values(sessionID, session.OriginalMaxAge, session.Expires, session.Secure, session.HttpOnly, session.Path, session.SameSite, "").ToSql()

	_, err := d.sqlDB.Exec(retrievedSql, inserts...)

	if err != nil {
		return err
	}

	return nil
}

func (d MysqlDB) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	var retrievedSql, args, _ = mysql.Select("*").From("padRev").Where(sq.Eq{"id": padId}).Where(sq.Eq{"rev": rev}).ToSql()

	query, err := d.sqlDB.Query(retrievedSql, args...)
	if err != nil {
		println("Error getting revision", err)
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

func (d MysqlDB) DoesPadExist(padID string) (*bool, error) {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) RemoveSessionById(sid string) error {

	var foundSession, err = d.GetSessionById(sid)
	if err != nil {
		return err
	}

	if foundSession == nil {
		return errors.New(SessionNotFoundError)
	}

	resultedSQL, args, err := mysql.Delete("sessionstorage").Where(sq.Eq{"id": sid}).ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return err
	}

	return nil
}

func (d MysqlDB) CreatePad(padID string, padDB db.PadDB) error {

	_, notFound := d.GetPad(padID)

	var marshalled, err = json.Marshal(padDB)

	if err != nil {
		return err
	}

	var resultedSQL string
	var args []interface{}
	var err1 error

	if notFound != nil {
		resultedSQL, args, err1 = mysql.
			Insert("pad").
			Columns("id", "data").
			Values(padID, string(marshalled)).ToSql()
	} else {
		resultedSQL, args, err1 = mysql.
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

func (d MysqlDB) GetPadIds() (*[]string, error) {
	var padIds []string
	var resultedSQL, _, err = mysql.
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

func (d MysqlDB) SaveRevision(padId string, rev int, changeset string, text db.AText, pool db.RevPool, authorId *string, timestamp int64) error {
	exists, err := d.DoesPadExist(padId)
	if err != nil {
		return err
	}

	if !*exists {
		return errors.New(PadDoesNotExistError)
	}

	marshalled, err := json.Marshal(pool)
	if err != nil {
		return fmt.Errorf("failed to marshal pool: %v", err)
	}

	toSql, i, err := mysql.Insert("padRev").
		Columns("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp", "pool").
		Values(padId, rev, changeset, text.Text, text.Attribs, authorId, timestamp, string(marshalled)).
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

func (d MysqlDB) GetPad(padID string) (*db.PadDB, error) {

	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) GetReadonlyPad(padId string) (*string, error) {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) CreatePad2ReadOnly(padId string, readonlyId string) error {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) CreateReadOnly2Pad(padId string, readonlyId string) error {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) GetReadOnly2Pad(id string) (*string, error) {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) SetAuthorByToken(token, authorId string) error {
	var resulltedSQL, arg, _ = mysql.
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
func (d MysqlDB) GetAuthor(author string) (*db.AuthorDB, error) {

	var resultedSQL, args, err = mysql.Select("*").
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

func (d MysqlDB) GetAuthorByToken(token string) (*string, error) {
	var resultedSQL, args, err = mysql.
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

func (d MysqlDB) SaveAuthor(author db.AuthorDB) error {
	if author.ID == "" {
		return errors.New("author ID is empty")
	}
	var foundAuthor, err = d.GetAuthor(author.ID)
	if err != nil && err.Error() != AuthorNotFoundError {
		return err
	}

	if foundAuthor == nil {
		var resultedSQL, i, err = mysql.
			Insert("globalAuthor").
			Columns("id", "colorId", "name", "timestamp").
			Values(author.ID, author.ColorId, author.Name, author.Timestamp).
			ToSql()
		_, err = d.sqlDB.Exec(resultedSQL, i...)
		if err != nil {
			return err
		}
	} else {
		var resultedSQL, i, err = mysql.
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

func (d MysqlDB) SaveAuthorName(authorId string, authorName string) error {
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

func (d MysqlDB) SaveAuthorColor(authorId string, authorColor string) error {
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

func (d MysqlDB) GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error) {
	padExists, err := d.DoesPadExist(padId)
	if err != nil {
		return nil, err
	}
	if !*padExists {
		return nil, errors.New(PadDoesNotExistError)
	}

	resultedSQL, args, err := mysql.
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
	defer query.Close()

	return nil, errors.New(PadRevisionNotFoundError)
}

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

// NewMySQLDB This function creates a new MysqlDB and returns a pointer to it.
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

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS pad (id VARCHAR(255) PRIMARY KEY, data TEXT)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS globalAuthor(id VARCHAR(255) PRIMARY KEY, colorId VARCHAR(50), name VARCHAR(255), timestamp BIGINT)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS padRev(id VARCHAR(255), rev INTEGER, changeset TEXT, atextText TEXT, atextAttribs TEXT, authorId VARCHAR(255), timestamp BIGINT, pool TEXT NOT NULL, PRIMARY KEY (id, rev), FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS token2author(token VARCHAR(255) PRIMARY KEY, author VARCHAR(255), FOREIGN KEY(author) REFERENCES globalAuthor(id) ON DELETE CASCADE)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS globalAuthorPads(id VARCHAR(255) NOT NULL, padID VARCHAR(255) NOT NULL, PRIMARY KEY(id, padID), FOREIGN KEY(id) REFERENCES globalAuthor(id) ON DELETE CASCADE, FOREIGN KEY(padID) REFERENCES pad(id) ON DELETE CASCADE)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS pad2readonly(id VARCHAR(255) PRIMARY KEY, data VARCHAR(255), FOREIGN KEY(id) REFERENCES pad(id) ON DELETE CASCADE)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS readonly2pad(id VARCHAR(255) PRIMARY KEY, data VARCHAR(255), FOREIGN KEY(data) REFERENCES pad(id) ON DELETE CASCADE)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS sessionstorage(id VARCHAR(255) PRIMARY KEY, originalMaxAge INTEGER, expires VARCHAR(255), secure BOOLEAN, httpOnly BOOLEAN, path VARCHAR(255), sameSite VARCHAR(50), connections TEXT)")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS groupPadGroup(id VARCHAR(255) PRIMARY KEY, name VARCHAR(255))")
	if err != nil {
		return nil, err
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS padChat(padId VARCHAR(255) NOT NULL, padHead INTEGER, chatText TEXT NOT NULL, authorId VARCHAR(255), timestamp BIGINT, PRIMARY KEY(padId, padHead), FOREIGN KEY(padId) REFERENCES pad(id) ON DELETE CASCADE)")
	if err != nil {
		return nil, err
	}

	return &MysqlDB{
		options: options,
		sqlDB:   sqlDb,
	}, nil
}

var _ DataStore = (*MysqlDB)(nil)

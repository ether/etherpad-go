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
)

type PostgresDB struct {
	options PostgresOptions
	sqlDB   *sql.DB
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
	var resultedSQL, args, err = psql.
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

func (d PostgresDB) GetSessionById(sessionID string) *session2.Session {
	var createdSQL, arr, _ = psql.Select("*").From("session").Where(sq.Eq{"id": sessionID}).ToSql()

	query, err := d.sqlDB.Query(createdSQL, arr...)

	if err != nil {
		panic(err)
	}

	var possibleSession *session2.Session

	for query.Next() {
		query.Scan(possibleSession)
	}

	return possibleSession
}

func (d PostgresDB) SetSessionById(sessionID string, session session2.Session) {
	var retrievedSql, inserts, _ = psql.Insert("session").Columns("id", "originalMaxAge", "expires", "secure", "httpOnly", "path", "sameSite", "connections").
		Values(sessionID, session.OriginalMaxAge, session.Expires, session.Secure, session.HttpOnly, session.Path, session.SameSite).ToSql()

	_, err := d.sqlDB.Exec(retrievedSql, inserts...)

	if err != nil {
		panic(err)
	}
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

	return nil, errors.New("revision not found")
}

func (d PostgresDB) DoesPadExist(padID string) bool {
	var resultedSQL, args, err = psql.
		Select("id").
		From("pad").
		Where(sq.Eq{"id": padID}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		println(err.Error())
	}

	for query != nil && query.Next() {
		return true
	}

	defer query.Close()
	return false
}

func (d PostgresDB) RemoveSessionById(sid string) *session2.Session {

	var foundSession = d.GetSessionById(sid)

	if foundSession == nil {
		return nil
	}

	var resultedSQL, args, err = psql.Delete("session").Where(sq.Eq{"id": sid}).ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		panic(err)
	}

	return foundSession
}

func (d PostgresDB) CreatePad(padID string, padDB db.PadDB) bool {

	_, notFound := d.GetPad(padID)

	var marshalled, err = json.Marshal(padDB)

	if err != nil {
		panic(err)
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
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		return false
	}

	return true
}

func (d PostgresDB) GetPadIds() []string {
	var padIds []string
	var resultedSQL, _, err = psql.
		Select("id").
		From("pad").
		Where(sq.Like{"id": "%"}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL)
	defer query.Close()
	if err != nil {
		panic(err)
	}

	for query.Next() {
		var padId string
		query.Scan(&padId)
		padIds = append(padIds, strings.TrimPrefix(padId, "pad:"))
	}

	return padIds
}

func (d PostgresDB) SaveRevision(padId string, rev int, changeset string, text apool.AText, pool apool.APool, authorId *string, timestamp int) error {
	toSql, i, err := psql.Insert("padRev").
		Columns("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp").
		Values(padId, rev, changeset, text.Text, text.Attribs, *authorId, timestamp).
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
	defer query.Close()
	if err != nil {
		return nil, err
	}

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
		return nil, errors.New("pad not found")
	}

	return padDB, nil
}

func (d PostgresDB) GetReadonlyPad(padId string) (*string, error) {
	var resultedSQL, args, err = psql.
		Select("id").
		From("pad2readonly").
		Where(sq.Eq{"id": padId}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	defer query.Close()
	if err != nil {
		panic(err)
	}

	var readonlyId string
	for query.Next() {
		query.Scan(&readonlyId)
		return &readonlyId, nil
	}

	return nil, errors.New("no read only id found")
}

func (d PostgresDB) CreatePad2ReadOnly(padId string, readonlyId string) {
	var resultedSQL, args, err = psql.
		Insert("pad2readonly").
		Columns("id", "data").
		Values(padId, readonlyId).
		ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		panic(err)
	}
}

func (d PostgresDB) CreateReadOnly2Pad(padId string, readonlyId string) {
	var resultedSQL, args, err = psql.
		Insert("readonly2pad").
		Columns("id", "data").
		Values(readonlyId, padId).
		ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		panic(err)
	}
}

func (d PostgresDB) GetReadOnly2Pad(id string) *string {
	var resultedSQL, _, err = psql.
		Select("id").
		From("readonly2pad").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL)
	defer query.Close()
	if err != nil {
		panic(err)
	}

	var padId string
	for query.Next() {
		query.Scan(&padId)
		return &padId
	}

	return nil
}

func (d PostgresDB) SetAuthorByToken(token, authorId string) error {
	var resulltedSQL, arg, _ = psql.
		Insert("token2author").
		Columns("token,author").
		Values(token, authorId).ToSql()

	_, err := d.sqlDB.Exec(resulltedSQL, arg...)

	if err != nil {
		panic(err)
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
	defer query.Close()
	if err != nil {
		return nil, err
	}

	var authorDB *db.AuthorDB
	for query.Next() {
		var authorCopy db.AuthorDB
		query.Scan(&authorCopy.ID, &authorCopy.ColorId, &authorCopy.Name, &authorCopy.Timestamp)
		authorDB = &authorCopy
	}

	if authorDB == nil {
		return nil, errors.New("author not found")
	}

	return authorDB, nil
}

func (d PostgresDB) GetAuthorByToken(token string) (*string, error) {
	var resultedSQL, args, err = psql.
		Select("author").
		From("token2author").
		Where(sq.Eq{"token": token}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	defer query.Close()
	if err != nil {
		panic(err)
	}

	var authorID string
	for query.Next() {
		query.Scan(&authorID)
	}

	if authorID == "" {
		return nil, errors.New("author for token not found")
	}

	return &authorID, nil
}

func (d PostgresDB) SaveAuthor(author db.AuthorDB) {
	if author.ID == "" {
		return
	}
	var foundAuthor, err = d.GetAuthor(author.ID)

	if foundAuthor == nil && err == nil {
		var resultedSQL, i, err = psql.
			Insert("globalAuthor").
			Columns("id", "colorId", "name", "timestamp").
			Values(author.ID, author.ColorId, author.Name, author.Timestamp).
			ToSql()
		_, err = d.sqlDB.Exec(resultedSQL, i...)
		if err != nil {
			panic(err)
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
			panic(err)
		}
	}
}

func (d PostgresDB) SaveAuthorName(authorId string, authorName string) {
	if authorId == "" {
		return
	}
	var authorString, err = d.GetAuthor(authorId)

	if err != nil || authorString == nil {
		return
	}

	authorString.Name = &authorName
	d.SaveAuthor(*authorString)
}

func (d PostgresDB) SaveAuthorColor(authorId string, authorColor string) {
	if authorId == "" {
		return
	}

	var authorString, err = d.GetAuthor(authorId)

	if err != nil || authorString == nil {
		return
	}

	authorString.ColorId = authorColor
	d.SaveAuthor(*authorString)
}

func (d PostgresDB) GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error) {
	var resultedSQL, args, err = psql.
		Select("*").
		From("padRev").
		Where(sq.Eq{"id": padId}).
		Where(sq.Eq{"rev": revNum}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		panic(err)
	}

	var padMetaData db.PadMetaData
	for query.Next() {
		err := query.Scan(&padMetaData.Id, &padMetaData.RevNum, &padMetaData.ChangeSet, &padMetaData.Atext, &padMetaData.AtextAttribs, &padMetaData.AuthorId, &padMetaData.Timestamp)
		if err != nil {
			return nil, err
		}
	}
	defer query.Close()

	return &padMetaData, nil
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
		panic(err)
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS pad (id TEXT PRIMARY KEY, data TEXT)")
	if err != nil {
		panic(err)
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS padRev(id TEXT, rev INTEGER, changeset TEXT, atextText TEXT, atextAttribs TEXT, authorId TEXT, timestamp BIGINT, PRIMARY KEY (id, rev))")
	if err != nil {
		panic(err)
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS token2author(token TEXT PRIMARY KEY, author TEXT)")

	if err != nil {
		panic(err)
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS globalAuthorPads(id TEXT NOT NULL, padID TEXT NOT NULL,  PRIMARY KEY(id, padID) )")

	if err != nil {
		panic(err.Error())
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS globalAuthor(id TEXT PRIMARY KEY, colorId TEXT, name TEXT, timestamp BIGINT)")

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS pad2readonly(id TEXT PRIMARY KEY, data TEXT)")

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS readonly2pad(id TEXT PRIMARY KEY, data TEXT)")

	if err != nil {
		panic(err.Error())
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS sessionstorage(id TEXT PRIMARY KEY, originalMaxAge INTEGER, expires TEXT, secure BOOLEAN, httpOnly BOOLEAN, path TEXT, sameSeite TEXT, connections TEXT)")

	if err != nil {
		panic(err.Error())
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS groupPadGroup(id TEXT PRIMARY KEY, name TEXT)")

	if err != nil {
		panic(err.Error())
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS padChat(padId TEXT NOT NULL, padHead INTEGER,  chatText TEXT NOT NULL, authorId TEXT, timestamp BIGINT, PRIMARY KEY(padId, padHead), FOREIGN KEY(padId) REFERENCES pad(id) ON DELETE CASCADE)")

	if err != nil {
		panic(err.Error())
	}

	return &PostgresDB{
		options: options,
		sqlDB:   sqlDb,
	}, nil
}

var _ DataStore = (*PostgresDB)(nil)

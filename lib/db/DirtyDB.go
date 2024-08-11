package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
	_ "modernc.org/sqlite"
	"os"
	"strings"
)

func init() {
	println("Init")
	os.Setenv("ETHERPAD_DB_TYPE", "memory")
}

type SQLiteDB struct {
	path  string
	sqlDB *sql.DB
}

func (d SQLiteDB) GetSessionById(sessionID string) *session2.Session {
	var createdSQL, arr, _ = sq.Select("*").From("session").Where(sq.Eq{"id": sessionID}).ToSql()

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

func (d SQLiteDB) SetSessionById(sessionID string, session session2.Session) {
	var retrievedSql, inserts, _ = sq.Insert("session").Columns("id", "originalMaxAge", "expires", "secure", "httpOnly", "path", "sameSite", "connections").
		Values(sessionID, session.OriginalMaxAge, session.Expires, session.Secure, session.HttpOnly, session.Path, session.SameSite).ToSql()

	_, err := d.sqlDB.Exec(retrievedSql, inserts...)

	if err != nil {
		panic(err)
	}
}

func (d SQLiteDB) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	query, err := d.sqlDB.Query("SELECT * FROM padRev WHERE id = ? AND rev = ?", padId, rev)
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

const padPrefix = "pad:%s"
const readOnlyPrefix = "readonly2pad:%s"
const authorPrefix = "author:%s"
const chatMessage = "pad:%s:chat:%s"
const globalAuthor = "globalAuthor:"
const group = "group:%s"
const mapper2group = "mapper2group:%s"
const revs = "pad:%s:revs:%d"
const pad2readonly = "pad2readonly:%s"
const readonly2pad = "readonly2pad:%s"
const session = "session:%s"
const sessionStorage = "sessionStorage:%s"

func (d SQLiteDB) DoesPadExist(padID string) bool {
	var resultedSQL, args, err = sq.
		Select("id").
		From("pad").
		Where(sq.Eq{"id": fmt.Sprintf(padPrefix, padID)}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		println(err.Error())
	}

	defer query.Close()
	return query.Next()
}

func (d SQLiteDB) RemoveSessionById(sid string) *session2.Session {

	var foundSession = d.GetSessionById(sid)

	if foundSession == nil {
		return nil
	}

	var resultedSQL, args, err = sq.Delete("session").Where(sq.Eq{"id": sid}).ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		panic(err)
	}

	return foundSession
}

func (d SQLiteDB) CreatePad(padID string, padDB db.PadDB) bool {

	_, notFound := d.GetPad(padID)

	var marshalled, err = json.Marshal(padDB)

	if err != nil {
		panic(err)
	}

	var resultedSQL string
	var args []interface{}
	var err1 error

	if notFound != nil {
		resultedSQL, args, err1 = sq.
			Insert("pad").
			Columns("id", "data").
			Values(fmt.Sprintf(padPrefix, padID), string(marshalled)).ToSql()
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
		return false
	}

	return true
}

func (d SQLiteDB) GetPadIds() []string {
	var padIds []string
	var resultedSQL, _, err = sq.
		Select("id").
		From("pad").
		Where(sq.Like{"id": "pad:%"}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL)
	if err != nil {
		panic(err)
	}

	for query.Next() {
		var padId string
		query.Scan(&padId)
		padIds = append(padIds, strings.TrimPrefix(padId, "pad:"))
	}

	defer query.Close()

	return padIds
}

func (d SQLiteDB) SaveRevision(padId string, rev int, changeset string, text apool.AText, pool apool.APool, authorId *string, timestamp int) error {
	toSql, i, err := sq.Insert("padRev").Columns("id", "rev", "changeset", "atextText", "atextAttribs", "authorId", "timestamp").
		Values(padId, rev, changeset, text.Text, text.Attribs, *authorId, timestamp).
		ToSql()

	if err != nil {
		return err
	}

	_, err = d.sqlDB.Exec(toSql, i)

	if err != nil {
		return err
	}

	return nil
}

func (d SQLiteDB) GetPad(padID string) (*db.PadDB, error) {

	var resultedSQL, args, err = sq.
		Select("data").
		From("pad").
		Where(sq.Eq{"id": fmt.Sprintf(padPrefix, padID)}).
		ToSql()

	if err != nil {
		return nil, err
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	defer query.Close()
	if err != nil {
		return nil, err
	}

	var padDB db.PadDB
	for query.Next() {
		var data string
		query.Scan(&data)
		err = json.Unmarshal([]byte(data), &padDB)
		if err != nil {
			return nil, err
		}
	}

	if padDB.ReadOnlyId == "" {
		return nil, errors.New("pad not found")
	}

	return &padDB, nil
}

func (d SQLiteDB) GetReadonlyPad(padId string) (*string, error) {
	var resultedSQL, args, err = sq.
		Select("id").
		From("pad").
		Where(sq.Eq{"id": fmt.Sprintf(pad2readonly, padId)}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		panic(err)
	}

	var readonlyId string
	for query.Next() {
		query.Scan(&readonlyId)
		return &readonlyId, nil
	}

	defer query.Close()

	return nil, errors.New("no read only id found")
}

func (d SQLiteDB) CreatePad2ReadOnly(padId string, readonlyId string) {
	var resultedSQL, args, err = sq.
		Insert("pad").
		Columns("id", "data").
		Values(fmt.Sprintf(pad2readonly, padId), readonlyId).
		ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)
	if err != nil {
		panic(err)
	}
}

func (d SQLiteDB) CreateReadOnly2Pad(padId string, readonlyId string) {
	var resultedSQL, args, err = sq.
		Insert("pad").
		Columns("id", "data").
		Values(fmt.Sprintf(readonly2pad, readonlyId), padId).
		ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, args...)

	if err != nil {
		panic(err)
	}
}

func (d SQLiteDB) GetReadOnly2Pad(id string) *string {
	var resultedSQL, _, err = sq.
		Select("id").
		From("pad").
		Where(sq.Eq{"id": fmt.Sprintf(readonly2pad, id)}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL)
	if err != nil {
		panic(err)
	}

	var padId string
	for query.Next() {
		query.Scan(&padId)
		return &padId
	}
	defer query.Close()

	return nil
}

func (d SQLiteDB) SetAuthorByToken(token, authorId string) error {
	var resulltedSQL, arg, _ = sq.
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
func (d SQLiteDB) GetAuthor(author string) (*db.AuthorDB, error) {
	var authorString, err = d.GetAuthor(author)

	if err != nil {
		return nil, err
	}

	return authorString, nil
}

func (d SQLiteDB) GetAuthorByToken(token string) (*string, error) {
	var resultedSQL, args, err = sq.
		Select("author").
		From("token2author").
		Where(sq.Eq{"id": token}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		panic(err)
	}

	var authorID *string
	for query.Next() {
		query.Scan(authorID)
		if err != nil {
			return nil, err
		}
	}
	defer query.Close()

	if authorID == nil {
		return nil, errors.New("author for token not found")
	}

	return authorID, nil
}

func (d SQLiteDB) SaveAuthor(author db.AuthorDB) {
	var marshalled, _ = json.Marshal(author)
	var resultedSQL, i, err = sq.
		Insert("pad").
		Columns("id", "data").
		Values(fmt.Sprintf(authorPrefix, author.ID), string(marshalled)).
		ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL, i...)
	if err != nil {
		panic(err)
	}
}

func (d SQLiteDB) SaveAuthorName(authorId string, authorName string) {
	var authorString, err = d.GetAuthor(authorId)

	if err != nil {
		return
	}

	authorString.Name = &authorName
	d.SaveAuthor(*authorString)
}

func (d SQLiteDB) SaveAuthorColor(authorId string, authorColor string) {
	var authorString, err = d.GetAuthor(authorId)

	if err != nil {
		return
	}

	authorString.ColorId = authorColor
	d.SaveAuthor(*authorString)
}

func (d SQLiteDB) GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error) {
	var resultedSQL, args, err = sq.
		Select("data").
		From("pad").
		Where(sq.Eq{"id": fmt.Sprintf(revs, padId, revNum)}).
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
		var data string
		query.Scan(&data)
		err = json.Unmarshal([]byte(data), &padMetaData)
		if err != nil {
			return nil, err
		}
	}
	defer query.Close()

	return &padMetaData, nil
}

// NewDirtyDB This function creates a new SQLiteDB and returns a pointer to it.
func NewDirtyDB(path string) (*SQLiteDB, error) {
	sqlDb, err := sql.Open("sqlite", path)
	if err != nil {
		panic(err)
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS pad (id TEXT PRIMARY KEY, data TEXT)")
	if err != nil {
		panic(err)
	}
	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS padRev(id TEXT, rev INTEGER, changeset TEXT, atextText TEXT, atextAttribs TEXT, authorId TEXT, timestamp INTEGER, PRIMARY KEY (id, rev))")
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

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS globalAuthor(id TEXT PRIMARY KEY, colorId INTEGER, name TEXT, timestamp BIGINT)")

	if err != nil {
		panic(err.Error())
	}

	_, err = sqlDb.Exec("CREATE TABLE IF NOT EXISTS sessionstorage(id TEXT PRIMARY KEY, originalMaxAge INTEGER, expires TEXT, secure BOOLEAN httpOnly BOOLEAN, path TEXT, sameSeite TEXT, connections TEXT)")

	return &SQLiteDB{
		path:  path,
		sqlDB: sqlDb,
	}, nil
}

var _ DataStore = (*SQLiteDB)(nil)

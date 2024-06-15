package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
	_ "modernc.org/sqlite"
	"strings"
)

type SQLiteDB struct {
	path  string
	sqlDB *sql.DB
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

	return query.Next()
}

func (d SQLiteDB) CreatePad(padID string, padDB db.PadDB) bool {
	var marshalled, err = json.Marshal(padDB)

	if err != nil {
		panic(err)
	}

	var resultedSQL, args, err1 = sq.
		Insert("pad").
		Columns("id", "data").
		Values(fmt.Sprintf(padPrefix, padID), string(marshalled)).ToSql()

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

	return padIds
}

func (d SQLiteDB) SaveRevision(padId string, rev int, changeset string, text apool.AText, pool apool.APool, authorId *string, timestamp int) {

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
		Where(sq.Eq{"id": fmt.Sprintf(readOnlyPrefix, padId)}).
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

	return nil, nil
}

func (d SQLiteDB) CreatePad2ReadOnly(padId string, readonlyId string) {
	var resultedSQL, _, err = sq.
		Insert("pad").
		Columns("id", "data").
		Values(fmt.Sprintf(pad2readonly, padId), readonlyId).
		ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL)
	if err != nil {
		panic(err)
	}
}

func (d SQLiteDB) CreateReadOnly2Pad(padId string, readonlyId string) {
	var resultedSQL, _, err = sq.
		Insert("pad").
		Columns("id", "data").
		Values(fmt.Sprintf(readonly2pad, padId), readonlyId).
		ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL)
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

	return nil
}

func (d SQLiteDB) GetAuthor(author string) (*db.AuthorDB, error) {
	var authorString, err = d.GetAuthorByMapperKeyAndMapperValue("authorID", author)

	if err != nil {
		return nil, err
	}

	return authorString, nil
}

func (d SQLiteDB) GetAuthorByMapperKeyAndMapperValue(key string, value string) (*db.AuthorDB, error) {
	var resultedSQL, args, err = sq.
		Select("data").
		From("pad").
		Where(sq.Eq{"id": fmt.Sprintf(authorPrefix, value)}).
		ToSql()

	if err != nil {
		panic(err)
	}

	query, err := d.sqlDB.Query(resultedSQL, args...)
	if err != nil {
		panic(err)
	}

	var authorDB db.AuthorDB
	for query.Next() {
		var data string
		query.Scan(&data)
		err = json.Unmarshal([]byte(data), &authorDB)
		if err != nil {
			return nil, err
		}
	}

	return &authorDB, nil
}

func (d SQLiteDB) SaveAuthor(author db.AuthorDB) {
	var marshalled, _ = json.Marshal(author)
	var resultedSQL, _, err = sq.
		Insert("pad").
		Columns("id", "data").
		Values(fmt.Sprintf(authorPrefix, author.ID), string(marshalled)).
		ToSql()

	if err != nil {
		panic(err)
	}

	_, err = d.sqlDB.Exec(resultedSQL)
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

	return &SQLiteDB{
		path:  path,
		sqlDB: sqlDb,
	}, nil
}

var _ DataStore = (*SQLiteDB)(nil)

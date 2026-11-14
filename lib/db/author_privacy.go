package db

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	modeldb "github.com/ether/etherpad-go/lib/models/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuthorRow is an author as the admin's author list shows it.
type AuthorRow struct {
	ID       string
	Name     *string
	ColorId  string
	Mapper   *string
	LastSeen int64 // ms, 0 unknown
	Erased   bool
	ErasedAt int64 // ms, 0 not erased
}

// AuthorSearch selects authors for the admin's list.
type AuthorSearch struct {
	// Pattern is matched, case-insensitively, against the id, name and
	// mapper; "" matches every author.
	Pattern       string
	IncludeErased bool
	// SortBy is "name" or "lastSeen".
	SortBy    string
	Ascending bool
	// Cap is the most rows returned.
	Cap int
}

// TokenTransfer is a one-time code that moves an author's identity (its
// token cookie) and preferences to another browser.
type TokenTransfer struct {
	ID        string
	Token     string
	Prefs     string
	CreatedAt time.Time
}

// ErrTokenTransferNotFound is returned for an unknown or redeemed code.
var ErrTokenTransferNotFound = errors.New("token transfer not found")

// AuthorPrivacyMethods are the author data of erasure (GDPR Art. 17), the
// admin's author list and the token transfer.
type AuthorPrivacyMethods interface {
	// GetAuthorByMapper returns the author created for mapper, or "".
	GetAuthorByMapper(mapper string) (string, error)
	SetAuthorMapper(authorId string, mapper string) error
	// RemoveMapperOfAuthor drops the mapper and reports whether there was one.
	RemoveMapperOfAuthor(authorId string) (bool, error)
	// TouchAuthor records that the author was seen at (ms).
	TouchAuthor(authorId string, at int64) error
	// GetAuthorRow returns the author, or nil.
	GetAuthorRow(authorId string) (*AuthorRow, error)
	MarkAuthorErased(authorId string, at int64) error
	CountChatMessagesOfAuthor(authorId string) (int, error)
	SearchAuthors(s AuthorSearch) ([]AuthorRow, error)

	CreateTokenTransfer(t TokenTransfer) error
	// TakeTokenTransfer removes and returns the transfer: it is redeemed at
	// most once, also by two requests at the same time.
	TakeTokenTransfer(id string) (*TokenTransfer, error)
	// RemoveTokenTransfersBefore drops transfers created before t.
	RemoveTokenTransfersBefore(t time.Time) error
}

// querier runs SQL with ? placeholders on database/sql or pgx.
type querier interface {
	exec(q string, args ...any) (int64, error)
	query(q string, args []any, row func(scan func(...any) error) error) error
}

type sqlQuerier struct{ db *sql.DB }

func (s sqlQuerier) exec(q string, args ...any) (int64, error) {
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s sqlQuerier) query(q string, args []any, row func(scan func(...any) error) error) error {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := row(rows.Scan); err != nil {
			return err
		}
	}
	return rows.Err()
}

type pgQuerier struct{ pool *pgxpool.Pool }

// dollar turns ? placeholders into $1, $2, … (the queries here have no ?
// in literals).
func dollar(q string) string {
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (p pgQuerier) exec(q string, args ...any) (int64, error) {
	tag, err := p.pool.Exec(context.Background(), dollar(q), args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (p pgQuerier) query(q string, args []any, row func(scan func(...any) error) error) error {
	rows, err := p.pool.Query(context.Background(), dollar(q), args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := row(rows.Scan); err != nil {
			return err
		}
	}
	return rows.Err()
}

// The table and column names are unquoted: Postgres folds them to the
// lower case it created them with, SQLite and MySQL keep them.
const authorRowColumns = `id, colorId, name, mapper, last_seen, erased, erased_at`

func scanAuthorRow(scan func(...any) error) (AuthorRow, error) {
	var r AuthorRow
	var color, name, mapper sql.NullString
	var lastSeen, erasedAt sql.NullInt64
	var erased sql.NullBool
	if err := scan(&r.ID, &color, &name, &mapper, &lastSeen, &erased, &erasedAt); err != nil {
		return r, err
	}
	r.ColorId = color.String
	if name.Valid {
		r.Name = &name.String
	}
	if mapper.Valid {
		r.Mapper = &mapper.String
	}
	r.LastSeen, r.Erased, r.ErasedAt = lastSeen.Int64, erased.Bool, erasedAt.Int64
	return r, nil
}

func getAuthorByMapper(q querier, mapper string) (string, error) {
	id := ""
	err := q.query(`SELECT id FROM globalAuthor WHERE mapper = ? ORDER BY id LIMIT 1`, []any{mapper}, func(scan func(...any) error) error {
		return scan(&id)
	})
	return id, err
}

func setAuthorMapper(q querier, authorId, mapper string) error {
	_, err := q.exec(`UPDATE globalAuthor SET mapper = ? WHERE id = ?`, mapper, authorId)
	return err
}

func removeMapperOfAuthor(q querier, authorId string) (bool, error) {
	n, err := q.exec(`UPDATE globalAuthor SET mapper = NULL WHERE id = ? AND mapper IS NOT NULL`, authorId)
	return n > 0, err
}

func touchAuthor(q querier, authorId string, at int64) error {
	_, err := q.exec(`UPDATE globalAuthor SET last_seen = ? WHERE id = ?`, at, authorId)
	return err
}

func getAuthorRow(q querier, authorId string) (*AuthorRow, error) {
	var out *AuthorRow
	err := q.query(`SELECT `+authorRowColumns+` FROM globalAuthor WHERE id = ?`, []any{authorId}, func(scan func(...any) error) error {
		r, err := scanAuthorRow(scan)
		out = &r
		return err
	})
	return out, err
}

func markAuthorErased(q querier, authorId string, at int64) error {
	_, err := q.exec(`UPDATE globalAuthor SET erased = ?, erased_at = ? WHERE id = ?`, true, at, authorId)
	return err
}

func countChatMessagesOfAuthor(q querier, authorId string) (int, error) {
	n := 0
	err := q.query(`SELECT COUNT(*) FROM padChat WHERE authorId = ?`, []any{authorId}, func(scan func(...any) error) error {
		return scan(&n)
	})
	return n, err
}

// likeEscape makes a user's pattern literal for LIKE ... ESCAPE '!'.
func likeEscape(s string) string {
	r := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	return "%" + r.Replace(strings.ToLower(s)) + "%"
}

func searchAuthors(q querier, s AuthorSearch) ([]AuthorRow, error) {
	where := []string{}
	args := []any{}
	if !s.IncludeErased {
		where = append(where, "erased = ?")
		args = append(args, false)
	}
	if s.Pattern != "" {
		p := likeEscape(s.Pattern)
		where = append(where, "(LOWER(id) LIKE ? ESCAPE '!' OR LOWER(COALESCE(name, '')) LIKE ? ESCAPE '!' OR LOWER(COALESCE(mapper, '')) LIKE ? ESCAPE '!')")
		args = append(args, p, p, p)
	}
	query := `SELECT ` + authorRowColumns + ` FROM globalAuthor`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	dir := "ASC"
	if !s.Ascending {
		dir = "DESC"
	}
	if s.SortBy == "lastSeen" {
		query += " ORDER BY COALESCE(last_seen, 0) " + dir + ", id " + dir
	} else {
		query += " ORDER BY LOWER(COALESCE(name, '')) " + dir + ", id " + dir
	}
	query += " LIMIT " + strconv.Itoa(max(s.Cap, 0))
	var out []AuthorRow
	err := q.query(query, args, func(scan func(...any) error) error {
		r, err := scanAuthorRow(scan)
		out = append(out, r)
		return err
	})
	return out, err
}

func createTokenTransfer(q querier, t TokenTransfer) error {
	_, err := q.exec(`INSERT INTO token_transfers (id, token, prefs, created_at) VALUES (?, ?, ?, ?)`,
		t.ID, t.Token, t.Prefs, t.CreatedAt.UnixMilli())
	return err
}

func takeTokenTransfer(q querier, id string) (*TokenTransfer, error) {
	var t *TokenTransfer
	err := q.query(`SELECT id, token, prefs, created_at FROM token_transfers WHERE id = ?`, []any{id}, func(scan func(...any) error) error {
		var tt TokenTransfer
		var created int64
		if err := scan(&tt.ID, &tt.Token, &tt.Prefs, &created); err != nil {
			return err
		}
		tt.CreatedAt = time.UnixMilli(created)
		t = &tt
		return nil
	})
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, ErrTokenTransferNotFound
	}
	// Whoever deletes the row redeems it: of two requests at once only one
	// deletes it.
	n, err := q.exec(`DELETE FROM token_transfers WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrTokenTransferNotFound
	}
	return t, nil
}

func removeTokenTransfersBefore(q querier, t time.Time) error {
	_, err := q.exec(`DELETE FROM token_transfers WHERE created_at < ?`, t.UnixMilli())
	return err
}

// --- the SQL stores ---

func (d SQLiteDB) q() querier   { return sqlQuerier{d.sqlDB} }
func (d MysqlDB) q() querier    { return sqlQuerier{d.sqlDB} }
func (d PostgresDB) q() querier { return pgQuerier{d.pool} }

func (d SQLiteDB) GetAuthorByMapper(m string) (string, error) { return getAuthorByMapper(d.q(), m) }
func (d SQLiteDB) SetAuthorMapper(id, m string) error         { return setAuthorMapper(d.q(), id, m) }
func (d SQLiteDB) RemoveMapperOfAuthor(id string) (bool, error) {
	return removeMapperOfAuthor(d.q(), id)
}
func (d SQLiteDB) TouchAuthor(id string, at int64) error      { return touchAuthor(d.q(), id, at) }
func (d SQLiteDB) GetAuthorRow(id string) (*AuthorRow, error) { return getAuthorRow(d.q(), id) }
func (d SQLiteDB) MarkAuthorErased(id string, at int64) error { return markAuthorErased(d.q(), id, at) }
func (d SQLiteDB) CountChatMessagesOfAuthor(id string) (int, error) {
	return countChatMessagesOfAuthor(d.q(), id)
}
func (d SQLiteDB) SearchAuthors(s AuthorSearch) ([]AuthorRow, error) { return searchAuthors(d.q(), s) }
func (d SQLiteDB) CreateTokenTransfer(t TokenTransfer) error         { return createTokenTransfer(d.q(), t) }
func (d SQLiteDB) TakeTokenTransfer(id string) (*TokenTransfer, error) {
	return takeTokenTransfer(d.q(), id)
}
func (d SQLiteDB) RemoveTokenTransfersBefore(t time.Time) error {
	return removeTokenTransfersBefore(d.q(), t)
}

func (d MysqlDB) GetAuthorByMapper(m string) (string, error) { return getAuthorByMapper(d.q(), m) }
func (d MysqlDB) SetAuthorMapper(id, m string) error         { return setAuthorMapper(d.q(), id, m) }
func (d MysqlDB) RemoveMapperOfAuthor(id string) (bool, error) {
	return removeMapperOfAuthor(d.q(), id)
}
func (d MysqlDB) TouchAuthor(id string, at int64) error      { return touchAuthor(d.q(), id, at) }
func (d MysqlDB) GetAuthorRow(id string) (*AuthorRow, error) { return getAuthorRow(d.q(), id) }
func (d MysqlDB) MarkAuthorErased(id string, at int64) error { return markAuthorErased(d.q(), id, at) }
func (d MysqlDB) CountChatMessagesOfAuthor(id string) (int, error) {
	return countChatMessagesOfAuthor(d.q(), id)
}
func (d MysqlDB) SearchAuthors(s AuthorSearch) ([]AuthorRow, error) { return searchAuthors(d.q(), s) }
func (d MysqlDB) CreateTokenTransfer(t TokenTransfer) error         { return createTokenTransfer(d.q(), t) }
func (d MysqlDB) TakeTokenTransfer(id string) (*TokenTransfer, error) {
	return takeTokenTransfer(d.q(), id)
}
func (d MysqlDB) RemoveTokenTransfersBefore(t time.Time) error {
	return removeTokenTransfersBefore(d.q(), t)
}

func (d PostgresDB) GetAuthorByMapper(m string) (string, error) { return getAuthorByMapper(d.q(), m) }
func (d PostgresDB) SetAuthorMapper(id, m string) error         { return setAuthorMapper(d.q(), id, m) }
func (d PostgresDB) RemoveMapperOfAuthor(id string) (bool, error) {
	return removeMapperOfAuthor(d.q(), id)
}
func (d PostgresDB) TouchAuthor(id string, at int64) error      { return touchAuthor(d.q(), id, at) }
func (d PostgresDB) GetAuthorRow(id string) (*AuthorRow, error) { return getAuthorRow(d.q(), id) }
func (d PostgresDB) MarkAuthorErased(id string, at int64) error {
	return markAuthorErased(d.q(), id, at)
}
func (d PostgresDB) CountChatMessagesOfAuthor(id string) (int, error) {
	return countChatMessagesOfAuthor(d.q(), id)
}
func (d PostgresDB) SearchAuthors(s AuthorSearch) ([]AuthorRow, error) {
	return searchAuthors(d.q(), s)
}
func (d PostgresDB) CreateTokenTransfer(t TokenTransfer) error { return createTokenTransfer(d.q(), t) }
func (d PostgresDB) TakeTokenTransfer(id string) (*TokenTransfer, error) {
	return takeTokenTransfer(d.q(), id)
}
func (d PostgresDB) RemoveTokenTransfersBefore(t time.Time) error {
	return removeTokenTransfersBefore(d.q(), t)
}

// --- the memory store ---

// memoryAuthorExtra holds what AuthorDB does not.
type memoryAuthorExtra struct {
	mapper   *string
	lastSeen int64
	erased   bool
	erasedAt int64
}

func (m *MemoryDataStore) extra(id string) memoryAuthorExtra {
	if m.authorExtra == nil {
		m.authorExtra = map[string]memoryAuthorExtra{}
	}
	return m.authorExtra[id]
}

func (m *MemoryDataStore) GetAuthorByMapper(mapper string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := []string{}
	for id, e := range m.authorExtra {
		if _, ok := m.authorStore[id]; ok && e.mapper != nil && *e.mapper == mapper {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "", nil
	}
	return ids[0], nil
}

func (m *MemoryDataStore) SetAuthorMapper(authorId, mapper string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.extra(authorId)
	e.mapper = &mapper
	m.authorExtra[authorId] = e
	return nil
}

func (m *MemoryDataStore) RemoveMapperOfAuthor(authorId string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.extra(authorId)
	had := e.mapper != nil
	e.mapper = nil
	m.authorExtra[authorId] = e
	return had, nil
}

func (m *MemoryDataStore) TouchAuthor(authorId string, at int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.extra(authorId)
	e.lastSeen = at
	m.authorExtra[authorId] = e
	return nil
}

func (m *MemoryDataStore) rowLocked(id string, a modeldb.AuthorDB) AuthorRow {
	e := m.extra(id)
	return AuthorRow{ID: id, Name: a.Name, ColorId: a.ColorId, Mapper: e.mapper, LastSeen: e.lastSeen, Erased: e.erased, ErasedAt: e.erasedAt}
}

func (m *MemoryDataStore) GetAuthorRow(authorId string) (*AuthorRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.authorStore[authorId]
	if !ok {
		return nil, nil
	}
	r := m.rowLocked(authorId, a)
	return &r, nil
}

func (m *MemoryDataStore) MarkAuthorErased(authorId string, at int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.extra(authorId)
	e.erased, e.erasedAt = true, at
	m.authorExtra[authorId] = e
	return nil
}

func (m *MemoryDataStore) CountChatMessagesOfAuthor(authorId string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.chatPads {
		if c.AuthorId != nil && *c.AuthorId == authorId {
			n++
		}
	}
	return n, nil
}

func (m *MemoryDataStore) SearchAuthors(s AuthorSearch) ([]AuthorRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := strings.ToLower(s.Pattern)
	var rows []AuthorRow
	for id, a := range m.authorStore {
		r := m.rowLocked(id, a)
		if r.Erased && !s.IncludeErased {
			continue
		}
		if p != "" {
			hay := strings.ToLower(id)
			if r.Name != nil {
				hay += "\x00" + strings.ToLower(*r.Name)
			}
			if r.Mapper != nil {
				hay += "\x00" + strings.ToLower(*r.Mapper)
			}
			if !strings.Contains(hay, p) {
				continue
			}
		}
		rows = append(rows, r)
	}
	key := func(r AuthorRow) string {
		if r.Name == nil {
			return ""
		}
		return strings.ToLower(*r.Name)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if s.Ascending {
			a, b = rows[i], rows[j]
		} else {
			a, b = rows[j], rows[i]
		}
		if s.SortBy == "lastSeen" {
			if a.LastSeen != b.LastSeen {
				return a.LastSeen < b.LastSeen
			}
		} else if key(a) != key(b) {
			return key(a) < key(b)
		}
		return a.ID < b.ID
	})
	if len(rows) > s.Cap {
		rows = rows[:max(s.Cap, 0)]
	}
	return rows, nil
}

func (m *MemoryDataStore) CreateTokenTransfer(t TokenTransfer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tokenTransfers == nil {
		m.tokenTransfers = map[string]TokenTransfer{}
	}
	m.tokenTransfers[t.ID] = t
	return nil
}

func (m *MemoryDataStore) TakeTokenTransfer(id string) (*TokenTransfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tokenTransfers[id]
	if !ok {
		return nil, ErrTokenTransferNotFound
	}
	delete(m.tokenTransfers, id)
	return &t, nil
}

func (m *MemoryDataStore) RemoveTokenTransfersBefore(before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.tokenTransfers {
		if t.CreatedAt.Before(before) {
			delete(m.tokenTransfers, id)
		}
	}
	return nil
}

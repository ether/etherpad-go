package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// Token is an API key (and later an OAuth token) of an account. Its rights
// are those of the account, cut down to its scopes and pad pattern
// (lib/tokens).
type Token struct {
	ID        string
	AccountID string
	Name      string
	// Hash is the SHA-256 of the key, hex; the key itself is not stored.
	Hash string
	// Prefix is the start of the key, to tell keys apart in a list.
	Prefix string
	// Scopes are the scopes the token may use; "*" means all of them.
	Scopes []string
	// PadPattern limits the token to matching pads; "" means all pads.
	PadPattern string
	// ExpiresAt nil: never.
	ExpiresAt *time.Time
	// MaxExpiresAt is the latest a replaced token may work: its expiry
	// before the rotation (nil: it had none). A grace period never goes
	// past it.
	MaxExpiresAt *time.Time
	RevokedAt    *time.Time
	// ReplacedBy is the token a rotation made from this one; Predecessor
	// the one it was made from.
	ReplacedBy  string
	Predecessor string
	// ClientID is the OAuth client of an OAuth token, "" for API keys.
	ClientID   string
	CreatedBy  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	LastUsedIP string
	// Limits are the token's own limits, on top of those of the roles.
	Limits map[string]int64
}

// ErrTokenNotFound is returned when a token does not exist.
var ErrTokenNotFound = errors.New("token not found")

// TokenMethods store tokens.
type TokenMethods interface {
	CreateToken(t Token) error
	GetToken(id string) (*Token, error)
	GetTokenByHash(hash string) (*Token, error)
	// ListTokens lists the tokens of an account, or all with "", oldest
	// first.
	ListTokens(accountID string) ([]Token, error)
	// UpdateToken writes the changeable fields: name, scopes, pad pattern,
	// expiry and its cap, revocation, rotation and limits.
	UpdateToken(t Token) error
	// TouchToken records a use.
	TouchToken(id string, at time.Time, ip string) error
}

const tokenColumns = `id, account_id, name, hash, prefix, scopes, pad_pattern, expires_at, revoked_at, replaced_by, predecessor,
	client_id, created_by, created_at, last_used_at, last_used_ip, limits, max_expires_at`

func tokenArgs(t Token) ([]any, error) {
	scopes, err := json.Marshal(nonNil(t.Scopes))
	if err != nil {
		return nil, err
	}
	limits := t.Limits
	if limits == nil {
		limits = map[string]int64{}
	}
	l, err := json.Marshal(limits)
	if err != nil {
		return nil, err
	}
	return []any{t.ID, t.AccountID, t.Name, t.Hash, t.Prefix, string(scopes), t.PadPattern, millis(t.ExpiresAt),
		millis(t.RevokedAt), t.ReplacedBy, t.Predecessor, t.ClientID, t.CreatedBy, t.CreatedAt.UnixMilli(),
		millis(t.LastUsedAt), t.LastUsedIP, string(l), millis(t.MaxExpiresAt)}, nil
}

func scanToken(scan func(...any) error) (*Token, error) {
	var t Token
	var scopes, limits string
	var expires, revoked, lastUsed, maxExpires sql.NullInt64
	var created int64
	if err := scan(&t.ID, &t.AccountID, &t.Name, &t.Hash, &t.Prefix, &scopes, &t.PadPattern, &expires, &revoked,
		&t.ReplacedBy, &t.Predecessor, &t.ClientID, &t.CreatedBy, &created, &lastUsed, &t.LastUsedIP, &limits, &maxExpires); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(scopes), &t.Scopes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(limits), &t.Limits); err != nil {
		return nil, err
	}
	t.ExpiresAt, t.RevokedAt, t.LastUsedAt = timeOrNil(expires), timeOrNil(revoked), timeOrNil(lastUsed)
	t.MaxExpiresAt = timeOrNil(maxExpires)
	t.CreatedAt = time.UnixMilli(created)
	return &t, nil
}

// --- MySQL and SQLite ---

type sqlTokens struct{ db sqlRunner }

func (s sqlTokens) CreateToken(t Token) error {
	args, err := tokenArgs(t)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO tokens (`+tokenColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, args...)
	if err != nil && isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (s sqlTokens) get(where string, arg any) (*Token, error) {
	t, err := scanToken(s.db.QueryRow(`SELECT `+tokenColumns+` FROM tokens WHERE `+where+` = ?`, arg).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

func (s sqlTokens) GetToken(id string) (*Token, error)         { return s.get("id", id) }
func (s sqlTokens) GetTokenByHash(hash string) (*Token, error) { return s.get("hash", hash) }

func (s sqlTokens) ListTokens(accountID string) ([]Token, error) {
	q, args := `SELECT `+tokenColumns+` FROM tokens ORDER BY created_at, id`, []any{}
	if accountID != "" {
		q, args = `SELECT `+tokenColumns+` FROM tokens WHERE account_id = ? ORDER BY created_at, id`, []any{accountID}
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Token{}
	for rows.Next() {
		t, err := scanToken(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s sqlTokens) UpdateToken(t Token) error {
	args, err := tokenArgs(t)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE tokens SET name = ?, scopes = ?, pad_pattern = ?, expires_at = ?, revoked_at = ?,
		replaced_by = ?, predecessor = ?, limits = ?, max_expires_at = ? WHERE id = ?`,
		args[2], args[5], args[6], args[7], args[8], args[9], args[10], args[16], args[17], t.ID)
	return changedOneOrExists(res, err, ErrTokenNotFound, func() (bool, error) {
		x, err := s.GetToken(t.ID)
		return x != nil, err
	})
}

func (s sqlTokens) TouchToken(id string, at time.Time, ip string) error {
	_, err := s.db.Exec(`UPDATE tokens SET last_used_at = ?, last_used_ip = ? WHERE id = ?`, at.UnixMilli(), ip, id)
	return err
}

// changedOneOrExists is changedOneOr for updates, where MySQL reports no
// changed row when the values stayed the same: then the row is looked for.
func changedOneOrExists(res sql.Result, err error, notFound error, exists func() (bool, error)) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	ok, err := exists()
	if err != nil {
		return err
	}
	if !ok {
		return notFound
	}
	return nil
}

// --- Postgres ---

type pgTokens struct{ d PostgresDB }

func (p pgTokens) CreateToken(t Token) error {
	args, err := tokenArgs(t)
	if err != nil {
		return err
	}
	_, err = p.d.pool.Exec(context.Background(), `INSERT INTO tokens (`+tokenColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`, args...)
	return pgConflict(err)
}

func (p pgTokens) get(where string, arg any) (*Token, error) {
	t, err := scanToken(p.d.pool.QueryRow(context.Background(),
		`SELECT `+tokenColumns+` FROM tokens WHERE `+where+` = $1`, arg).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

func (p pgTokens) GetToken(id string) (*Token, error)         { return p.get("id", id) }
func (p pgTokens) GetTokenByHash(hash string) (*Token, error) { return p.get("hash", hash) }

func (p pgTokens) ListTokens(accountID string) ([]Token, error) {
	q, args := `SELECT `+tokenColumns+` FROM tokens ORDER BY created_at, id`, []any{}
	if accountID != "" {
		q, args = `SELECT `+tokenColumns+` FROM tokens WHERE account_id = $1 ORDER BY created_at, id`, []any{accountID}
	}
	rows, err := p.d.pool.Query(context.Background(), q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Token{}
	for rows.Next() {
		t, err := scanToken(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (p pgTokens) UpdateToken(t Token) error {
	args, err := tokenArgs(t)
	if err != nil {
		return err
	}
	tag, err := p.d.pool.Exec(context.Background(), `UPDATE tokens SET name = $1, scopes = $2, pad_pattern = $3,
		expires_at = $4, revoked_at = $5, replaced_by = $6, predecessor = $7, limits = $8, max_expires_at = $9 WHERE id = $10`,
		args[2], args[5], args[6], args[7], args[8], args[9], args[10], args[16], args[17], t.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func (p pgTokens) TouchToken(id string, at time.Time, ip string) error {
	_, err := p.d.pool.Exec(context.Background(), `UPDATE tokens SET last_used_at = $1, last_used_ip = $2 WHERE id = $3`,
		at.UnixMilli(), ip, id)
	return err
}

func (d MysqlDB) tokens() sqlTokens   { return sqlTokens{db: d.sqlDB} }
func (d SQLiteDB) tokens() sqlTokens  { return sqlTokens{db: d.sqlDB} }
func (d PostgresDB) tokens() pgTokens { return pgTokens{d: d} }

func (d MysqlDB) CreateToken(t Token) error               { return d.tokens().CreateToken(t) }
func (d MysqlDB) GetToken(id string) (*Token, error)      { return d.tokens().GetToken(id) }
func (d MysqlDB) GetTokenByHash(h string) (*Token, error) { return d.tokens().GetTokenByHash(h) }
func (d MysqlDB) ListTokens(accountID string) ([]Token, error) {
	return d.tokens().ListTokens(accountID)
}
func (d MysqlDB) UpdateToken(t Token) error { return d.tokens().UpdateToken(t) }
func (d MysqlDB) TouchToken(id string, at time.Time, ip string) error {
	return d.tokens().TouchToken(id, at, ip)
}

func (d SQLiteDB) CreateToken(t Token) error               { return d.tokens().CreateToken(t) }
func (d SQLiteDB) GetToken(id string) (*Token, error)      { return d.tokens().GetToken(id) }
func (d SQLiteDB) GetTokenByHash(h string) (*Token, error) { return d.tokens().GetTokenByHash(h) }
func (d SQLiteDB) ListTokens(accountID string) ([]Token, error) {
	return d.tokens().ListTokens(accountID)
}
func (d SQLiteDB) UpdateToken(t Token) error { return d.tokens().UpdateToken(t) }
func (d SQLiteDB) TouchToken(id string, at time.Time, ip string) error {
	return d.tokens().TouchToken(id, at, ip)
}

func (d PostgresDB) CreateToken(t Token) error               { return d.tokens().CreateToken(t) }
func (d PostgresDB) GetToken(id string) (*Token, error)      { return d.tokens().GetToken(id) }
func (d PostgresDB) GetTokenByHash(h string) (*Token, error) { return d.tokens().GetTokenByHash(h) }
func (d PostgresDB) ListTokens(accountID string) ([]Token, error) {
	return d.tokens().ListTokens(accountID)
}
func (d PostgresDB) UpdateToken(t Token) error { return d.tokens().UpdateToken(t) }
func (d PostgresDB) TouchToken(id string, at time.Time, ip string) error {
	return d.tokens().TouchToken(id, at, ip)
}

// --- Memory ---

func cloneToken(t Token) Token {
	t.Scopes = append([]string{}, t.Scopes...)
	limits := make(map[string]int64, len(t.Limits))
	for k, v := range t.Limits {
		limits[k] = v
	}
	t.Limits = limits
	for _, p := range []**time.Time{&t.ExpiresAt, &t.RevokedAt, &t.LastUsedAt, &t.MaxExpiresAt} {
		if *p != nil {
			v := **p
			*p = &v
		}
	}
	return t
}

func (m *MemoryDataStore) CreateToken(t Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tokens[t.ID]; ok {
		return ErrConflict
	}
	for _, x := range m.tokens {
		if x.Hash == t.Hash {
			return ErrConflict
		}
	}
	m.tokens[t.ID] = cloneToken(t)
	return nil
}

func (m *MemoryDataStore) GetToken(id string) (*Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tokens[id]
	if !ok {
		return nil, nil
	}
	c := cloneToken(t)
	return &c, nil
}

func (m *MemoryDataStore) GetTokenByHash(hash string) (*Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tokens {
		if t.Hash == hash {
			c := cloneToken(t)
			return &c, nil
		}
	}
	return nil, nil
}

func (m *MemoryDataStore) ListTokens(accountID string) ([]Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Token{}
	for _, t := range m.tokens {
		if accountID == "" || t.AccountID == accountID {
			out = append(out, cloneToken(t))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *MemoryDataStore) UpdateToken(t Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.tokens[t.ID]
	if !ok {
		return ErrTokenNotFound
	}
	n := cloneToken(t)
	cur.Name, cur.Scopes, cur.PadPattern, cur.ExpiresAt, cur.RevokedAt = n.Name, n.Scopes, n.PadPattern, n.ExpiresAt, n.RevokedAt
	cur.ReplacedBy, cur.Predecessor, cur.Limits, cur.MaxExpiresAt = n.ReplacedBy, n.Predecessor, n.Limits, n.MaxExpiresAt
	m.tokens[t.ID] = cur
	return nil
}

func (m *MemoryDataStore) TouchToken(id string, at time.Time, ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tokens[id]
	if !ok {
		return nil
	}
	t.LastUsedAt, t.LastUsedIP = &at, ip
	m.tokens[id] = t
	return nil
}

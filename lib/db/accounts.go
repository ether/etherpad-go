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

// Account is a person who logs in, a service account that only owns
// tokens, or the built-in anonymous account of visitors who are not
// logged in (lib/accounts).
type Account struct {
	ID          string
	Username    string
	DisplayName string
	Email       string
	// Kind is "human", "service" or "anonymous".
	Kind string
	// PasswordHash is empty for accounts without a password.
	PasswordHash       string
	MustChangePassword bool
	DisabledAt         *time.Time
	// AuthorID links the account to its pad author, "" until it wrote.
	AuthorID    string
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

// Role is a named set of scopes that can include other roles.
type Role struct {
	ID          string
	Name        string
	Description string
	Builtin     bool
	Scopes      []string
	// Includes are the ids of the roles this one includes.
	Includes []string
	// Limits by name (AI requests per day, ...).
	Limits map[string]int64
}

// RoleAssignment gives an account a role, for all pads (PadPattern "") or
// the pads matching PadPattern.
type RoleAssignment struct {
	AccountID  string
	RoleID     string
	PadPattern string
	CreatedBy  string
	CreatedAt  time.Time
}

// AuditEntry records a change to accounts, roles or assignments.
type AuditEntry struct {
	ID      string
	At      time.Time
	ActorID string
	Action  string
	Target  string
	Details string
}

var (
	// ErrAccountNotFound is returned for an unknown account id.
	ErrAccountNotFound = errors.New("account not found")
	// ErrRoleNotFound is returned for an unknown role id.
	ErrRoleNotFound = errors.New("role not found")
	// ErrAssignmentNotFound is returned for an unknown role assignment.
	ErrAssignmentNotFound = errors.New("role assignment not found")
)

// AccountMethods store accounts, roles, their assignments and the audit
// log. Create methods return ErrConflict for a taken id or username.
type AccountMethods interface {
	CreateAccount(a Account) error
	// GetAccount returns the account, or nil.
	GetAccount(id string) (*Account, error)
	// GetAccountByUsername returns the account, or nil.
	GetAccountByUsername(username string) (*Account, error)
	// ListAccounts returns all accounts, oldest first.
	ListAccounts() ([]Account, error)
	UpdateAccount(a Account) error
	// DeleteAccount removes the account and its assignments.
	DeleteAccount(id string) error

	CreateRole(r Role) error
	// GetRole returns the role, or nil.
	GetRole(id string) (*Role, error)
	// ListRoles returns all roles by name.
	ListRoles() ([]Role, error)
	UpdateRole(r Role) error
	// DeleteRole removes the role, its assignments and its inclusion in
	// other roles.
	DeleteRole(id string) error

	// AddRoleAssignment stores a; the same account, role and pattern
	// again is ErrConflict.
	AddRoleAssignment(a RoleAssignment) error
	RemoveRoleAssignment(accountID, roleID, padPattern string) error
	// ListRoleAssignments returns the assignments of the account, or of
	// all accounts for "".
	ListRoleAssignments(accountID string) ([]RoleAssignment, error)

	AppendAudit(e AuditEntry) error
	// ListAudit returns the newest entries first, at most limit.
	ListAudit(limit int) ([]AuditEntry, error)
}

func timeOrNil(ms sql.NullInt64) *time.Time {
	if !ms.Valid {
		return nil
	}
	t := time.UnixMilli(ms.Int64)
	return &t
}

const accountColumns = `id, username, display_name, email, kind, password_hash, must_change_password, disabled_at, author_id, created_at, last_login_at`

func accountArgs(a Account) []any {
	return []any{a.ID, a.Username, a.DisplayName, a.Email, a.Kind, a.PasswordHash, a.MustChangePassword,
		millis(a.DisabledAt), a.AuthorID, a.CreatedAt.UnixMilli(), millis(a.LastLoginAt)}
}

func scanAccount(scan func(...any) error) (*Account, error) {
	var a Account
	var disabled, lastLogin sql.NullInt64
	var created int64
	if err := scan(&a.ID, &a.Username, &a.DisplayName, &a.Email, &a.Kind, &a.PasswordHash, &a.MustChangePassword,
		&disabled, &a.AuthorID, &created, &lastLogin); err != nil {
		return nil, err
	}
	a.DisabledAt = timeOrNil(disabled)
	a.LastLoginAt = timeOrNil(lastLogin)
	a.CreatedAt = time.UnixMilli(created)
	return &a, nil
}

const roleColumns = `id, name, description, builtin, scopes, includes, limits`

func roleArgs(r Role) ([]any, error) {
	scopes, err := json.Marshal(nonNil(r.Scopes))
	if err != nil {
		return nil, err
	}
	includes, err := json.Marshal(nonNil(r.Includes))
	if err != nil {
		return nil, err
	}
	limits := r.Limits
	if limits == nil {
		limits = map[string]int64{}
	}
	lim, err := json.Marshal(limits)
	if err != nil {
		return nil, err
	}
	return []any{r.ID, r.Name, r.Description, r.Builtin, string(scopes), string(includes), string(lim)}, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func scanRole(scan func(...any) error) (*Role, error) {
	var r Role
	var scopes, includes, limits string
	if err := scan(&r.ID, &r.Name, &r.Description, &r.Builtin, &scopes, &includes, &limits); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(scopes), &r.Scopes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(includes), &r.Includes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(limits), &r.Limits); err != nil {
		return nil, err
	}
	return &r, nil
}

const assignmentColumns = `account_id, role_id, pad_pattern, created_by, created_at`

func scanAssignment(scan func(...any) error) (*RoleAssignment, error) {
	var a RoleAssignment
	var created int64
	if err := scan(&a.AccountID, &a.RoleID, &a.PadPattern, &a.CreatedBy, &created); err != nil {
		return nil, err
	}
	a.CreatedAt = time.UnixMilli(created)
	return &a, nil
}

const auditColumns = `id, at, actor_id, action, target, details`

func scanAudit(scan func(...any) error) (*AuditEntry, error) {
	var e AuditEntry
	var at int64
	if err := scan(&e.ID, &at, &e.ActorID, &e.Action, &e.Target, &e.Details); err != nil {
		return nil, err
	}
	e.At = time.UnixMilli(at)
	return &e, nil
}

// --- MySQL and SQLite (database/sql, ? placeholders) ---

type sqlAccounts struct {
	db sqlRunner
	// begin opens a transaction for the changes that touch several tables.
	begin func() (*sql.Tx, error)
}

func (s sqlAccounts) conflict(err error) error {
	if err != nil && isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (s sqlAccounts) CreateAccount(a Account) error {
	_, err := s.db.Exec(`INSERT INTO accounts (`+accountColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, accountArgs(a)...)
	return s.conflict(err)
}

func (s sqlAccounts) getAccount(where string, arg any) (*Account, error) {
	a, err := scanAccount(s.db.QueryRow(`SELECT `+accountColumns+` FROM accounts WHERE `+where+` = ?`, arg).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

func (s sqlAccounts) GetAccount(id string) (*Account, error) { return s.getAccount("id", id) }
func (s sqlAccounts) GetAccountByUsername(u string) (*Account, error) {
	return s.getAccount("username", u)
}

func (s sqlAccounts) ListAccounts() ([]Account, error) {
	rows, err := s.db.Query(`SELECT ` + accountColumns + ` FROM accounts ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		a, err := scanAccount(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (s sqlAccounts) exists(table, id string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE id = ?`, id).Scan(&n)
	return n > 0, err
}

func (s sqlAccounts) UpdateAccount(a Account) error {
	// MySQL counts only changed rows, so look first.
	if ok, err := s.exists("accounts", a.ID); err != nil || !ok {
		if err == nil {
			err = ErrAccountNotFound
		}
		return err
	}
	args := accountArgs(a)
	_, err := s.db.Exec(`UPDATE accounts SET username = ?, display_name = ?, email = ?, kind = ?, password_hash = ?,
		must_change_password = ?, disabled_at = ?, author_id = ?, created_at = ?, last_login_at = ? WHERE id = ?`,
		append(args[1:], a.ID)...)
	return s.conflict(err)
}

func (s sqlAccounts) DeleteAccount(id string) error {
	tx, err := s.begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	if err := changedOneOr(res, err, ErrAccountNotFound); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM role_assignments WHERE account_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func changedOneOr(res sql.Result, err error, notFound error) error {
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return notFound
	}
	return nil
}

func (s sqlAccounts) CreateRole(r Role) error {
	args, err := roleArgs(r)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO roles (`+roleColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`, args...)
	return s.conflict(err)
}

func (s sqlAccounts) GetRole(id string) (*Role, error) {
	r, err := scanRole(s.db.QueryRow(`SELECT `+roleColumns+` FROM roles WHERE id = ?`, id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return r, err
}

func (s sqlAccounts) ListRoles() ([]Role, error) {
	rows, err := s.db.Query(`SELECT ` + roleColumns + ` FROM roles ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Role{}
	for rows.Next() {
		r, err := scanRole(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s sqlAccounts) UpdateRole(r Role) error {
	if ok, err := s.exists("roles", r.ID); err != nil || !ok {
		if err == nil {
			err = ErrRoleNotFound
		}
		return err
	}
	args, err := roleArgs(r)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE roles SET name = ?, description = ?, builtin = ?, scopes = ?, includes = ?, limits = ? WHERE id = ?`,
		append(args[1:], r.ID)...)
	return s.conflict(err)
}

func (s sqlAccounts) DeleteRole(id string) error {
	roles, err := s.ListRoles()
	if err != nil {
		return err
	}
	tx, err := s.begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM roles WHERE id = ?`, id)
	if err := changedOneOr(res, err, ErrRoleNotFound); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM role_assignments WHERE role_id = ?`, id); err != nil {
		return err
	}
	for _, r := range withoutInclude(roles, id) {
		includes, _ := json.Marshal(nonNil(r.Includes))
		if _, err := tx.Exec(`UPDATE roles SET includes = ? WHERE id = ?`, string(includes), r.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// withoutInclude returns the roles that include id, with id taken out.
func withoutInclude(roles []Role, id string) []Role {
	var out []Role
	for _, r := range roles {
		kept := make([]string, 0, len(r.Includes))
		for _, inc := range r.Includes {
			if inc != id {
				kept = append(kept, inc)
			}
		}
		if len(kept) != len(r.Includes) && r.ID != id {
			r.Includes = kept
			out = append(out, r)
		}
	}
	return out
}

func (s sqlAccounts) AddRoleAssignment(a RoleAssignment) error {
	_, err := s.db.Exec(`INSERT INTO role_assignments (`+assignmentColumns+`) VALUES (?, ?, ?, ?, ?)`,
		a.AccountID, a.RoleID, a.PadPattern, a.CreatedBy, a.CreatedAt.UnixMilli())
	return s.conflict(err)
}

func (s sqlAccounts) RemoveRoleAssignment(accountID, roleID, padPattern string) error {
	res, err := s.db.Exec(`DELETE FROM role_assignments WHERE account_id = ? AND role_id = ? AND pad_pattern = ?`,
		accountID, roleID, padPattern)
	return changedOneOr(res, err, ErrAssignmentNotFound)
}

func (s sqlAccounts) ListRoleAssignments(accountID string) ([]RoleAssignment, error) {
	q := `SELECT ` + assignmentColumns + ` FROM role_assignments`
	var args []any
	if accountID != "" {
		q += ` WHERE account_id = ?`
		args = append(args, accountID)
	}
	rows, err := s.db.Query(q+` ORDER BY created_at, account_id, role_id, pad_pattern`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoleAssignment{}
	for rows.Next() {
		a, err := scanAssignment(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (s sqlAccounts) AppendAudit(e AuditEntry) error {
	_, err := s.db.Exec(`INSERT INTO audit_log (`+auditColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, e.At.UnixMilli(), e.ActorID, e.Action, e.Target, e.Details)
	return err
}

func (s sqlAccounts) ListAudit(limit int) ([]AuditEntry, error) {
	rows, err := s.db.Query(`SELECT `+auditColumns+` FROM audit_log ORDER BY at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		e, err := scanAudit(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// --- Postgres ---

type pgAccounts struct{ d PostgresDB }

func pgConflict(err error) error {
	if err != nil && isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (p pgAccounts) CreateAccount(a Account) error {
	_, err := p.d.pool.Exec(context.Background(),
		`INSERT INTO accounts (`+accountColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, accountArgs(a)...)
	return pgConflict(err)
}

func (p pgAccounts) getAccount(where string, arg any) (*Account, error) {
	a, err := scanAccount(p.d.pool.QueryRow(context.Background(),
		`SELECT `+accountColumns+` FROM accounts WHERE `+where+` = $1`, arg).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

func (p pgAccounts) GetAccount(id string) (*Account, error) { return p.getAccount("id", id) }
func (p pgAccounts) GetAccountByUsername(u string) (*Account, error) {
	return p.getAccount("username", u)
}

func (p pgAccounts) ListAccounts() ([]Account, error) {
	rows, err := p.d.pool.Query(context.Background(), `SELECT `+accountColumns+` FROM accounts ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		a, err := scanAccount(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (p pgAccounts) UpdateAccount(a Account) error {
	args := accountArgs(a)
	tag, err := p.d.pool.Exec(context.Background(), `UPDATE accounts SET username = $2, display_name = $3, email = $4,
		kind = $5, password_hash = $6, must_change_password = $7, disabled_at = $8, author_id = $9, created_at = $10,
		last_login_at = $11 WHERE id = $1`, args...)
	if err != nil {
		return pgConflict(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func (p pgAccounts) DeleteAccount(id string) error {
	ctx := context.Background()
	tx, err := p.d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAccountNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM role_assignments WHERE account_id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p pgAccounts) CreateRole(r Role) error {
	args, err := roleArgs(r)
	if err != nil {
		return err
	}
	_, err = p.d.pool.Exec(context.Background(),
		`INSERT INTO roles (`+roleColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7)`, args...)
	return pgConflict(err)
}

func (p pgAccounts) GetRole(id string) (*Role, error) {
	r, err := scanRole(p.d.pool.QueryRow(context.Background(), `SELECT `+roleColumns+` FROM roles WHERE id = $1`, id).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return r, err
}

func (p pgAccounts) ListRoles() ([]Role, error) {
	rows, err := p.d.pool.Query(context.Background(), `SELECT `+roleColumns+` FROM roles ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Role{}
	for rows.Next() {
		r, err := scanRole(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (p pgAccounts) UpdateRole(r Role) error {
	args, err := roleArgs(r)
	if err != nil {
		return err
	}
	tag, err := p.d.pool.Exec(context.Background(),
		`UPDATE roles SET name = $2, description = $3, builtin = $4, scopes = $5, includes = $6, limits = $7 WHERE id = $1`, args...)
	if err != nil {
		return pgConflict(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

func (p pgAccounts) DeleteRole(id string) error {
	roles, err := p.ListRoles()
	if err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := p.d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM role_assignments WHERE role_id = $1`, id); err != nil {
		return err
	}
	for _, r := range withoutInclude(roles, id) {
		includes, _ := json.Marshal(nonNil(r.Includes))
		if _, err := tx.Exec(ctx, `UPDATE roles SET includes = $1 WHERE id = $2`, string(includes), r.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p pgAccounts) AddRoleAssignment(a RoleAssignment) error {
	_, err := p.d.pool.Exec(context.Background(),
		`INSERT INTO role_assignments (`+assignmentColumns+`) VALUES ($1, $2, $3, $4, $5)`,
		a.AccountID, a.RoleID, a.PadPattern, a.CreatedBy, a.CreatedAt.UnixMilli())
	return pgConflict(err)
}

func (p pgAccounts) RemoveRoleAssignment(accountID, roleID, padPattern string) error {
	tag, err := p.d.pool.Exec(context.Background(),
		`DELETE FROM role_assignments WHERE account_id = $1 AND role_id = $2 AND pad_pattern = $3`, accountID, roleID, padPattern)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}

func (p pgAccounts) ListRoleAssignments(accountID string) ([]RoleAssignment, error) {
	q := `SELECT ` + assignmentColumns + ` FROM role_assignments`
	var args []any
	if accountID != "" {
		q += ` WHERE account_id = $1`
		args = append(args, accountID)
	}
	rows, err := p.d.pool.Query(context.Background(), q+` ORDER BY created_at, account_id, role_id, pad_pattern`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoleAssignment{}
	for rows.Next() {
		a, err := scanAssignment(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (p pgAccounts) AppendAudit(e AuditEntry) error {
	_, err := p.d.pool.Exec(context.Background(), `INSERT INTO audit_log (`+auditColumns+`) VALUES ($1, $2, $3, $4, $5, $6)`,
		e.ID, e.At.UnixMilli(), e.ActorID, e.Action, e.Target, e.Details)
	return err
}

func (p pgAccounts) ListAudit(limit int) ([]AuditEntry, error) {
	rows, err := p.d.pool.Query(context.Background(),
		`SELECT `+auditColumns+` FROM audit_log ORDER BY at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		e, err := scanAudit(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// --- Memory ---

type memAccounts struct {
	accounts    map[string]Account
	roles       map[string]Role
	assignments []RoleAssignment
	audit       []AuditEntry
}

func newMemAccounts() *memAccounts {
	return &memAccounts{accounts: map[string]Account{}, roles: map[string]Role{}}
}

func cloneRole(r Role) Role {
	r.Scopes = append([]string(nil), r.Scopes...)
	r.Includes = append([]string(nil), r.Includes...)
	limits := make(map[string]int64, len(r.Limits))
	for k, v := range r.Limits {
		limits[k] = v
	}
	r.Limits = limits
	if r.Scopes == nil {
		r.Scopes = []string{}
	}
	if r.Includes == nil {
		r.Includes = []string{}
	}
	return r
}

func (m *MemoryDataStore) CreateAccount(a Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.acc.accounts[a.ID]; ok {
		return ErrConflict
	}
	for _, x := range m.acc.accounts {
		if x.Username == a.Username {
			return ErrConflict
		}
	}
	m.acc.accounts[a.ID] = a
	return nil
}

func (m *MemoryDataStore) GetAccount(id string) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.acc.accounts[id]
	if !ok {
		return nil, nil
	}
	return &a, nil
}

func (m *MemoryDataStore) GetAccountByUsername(u string) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.acc.accounts {
		if a.Username == u {
			found := a
			return &found, nil
		}
	}
	return nil, nil
}

func (m *MemoryDataStore) ListAccounts() ([]Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Account, 0, len(m.acc.accounts))
	for _, a := range m.acc.accounts {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *MemoryDataStore) UpdateAccount(a Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.acc.accounts[a.ID]; !ok {
		return ErrAccountNotFound
	}
	for _, x := range m.acc.accounts {
		if x.ID != a.ID && x.Username == a.Username {
			return ErrConflict
		}
	}
	m.acc.accounts[a.ID] = a
	return nil
}

func (m *MemoryDataStore) DeleteAccount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.acc.accounts[id]; !ok {
		return ErrAccountNotFound
	}
	delete(m.acc.accounts, id)
	kept := m.acc.assignments[:0]
	for _, a := range m.acc.assignments {
		if a.AccountID != id {
			kept = append(kept, a)
		}
	}
	m.acc.assignments = kept
	return nil
}

func (m *MemoryDataStore) CreateRole(r Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.acc.roles[r.ID]; ok {
		return ErrConflict
	}
	for _, x := range m.acc.roles {
		if x.Name == r.Name {
			return ErrConflict
		}
	}
	m.acc.roles[r.ID] = cloneRole(r)
	return nil
}

func (m *MemoryDataStore) GetRole(id string) (*Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.acc.roles[id]
	if !ok {
		return nil, nil
	}
	c := cloneRole(r)
	return &c, nil
}

func (m *MemoryDataStore) ListRoles() ([]Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Role, 0, len(m.acc.roles))
	for _, r := range m.acc.roles {
		out = append(out, cloneRole(r))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *MemoryDataStore) UpdateRole(r Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.acc.roles[r.ID]; !ok {
		return ErrRoleNotFound
	}
	for _, x := range m.acc.roles {
		if x.ID != r.ID && x.Name == r.Name {
			return ErrConflict
		}
	}
	m.acc.roles[r.ID] = cloneRole(r)
	return nil
}

func (m *MemoryDataStore) DeleteRole(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.acc.roles[id]; !ok {
		return ErrRoleNotFound
	}
	delete(m.acc.roles, id)
	var roles []Role
	for _, r := range m.acc.roles {
		roles = append(roles, r)
	}
	for _, r := range withoutInclude(roles, id) {
		m.acc.roles[r.ID] = r
	}
	kept := m.acc.assignments[:0]
	for _, a := range m.acc.assignments {
		if a.RoleID != id {
			kept = append(kept, a)
		}
	}
	m.acc.assignments = kept
	return nil
}

func (m *MemoryDataStore) AddRoleAssignment(a RoleAssignment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.acc.assignments {
		if x.AccountID == a.AccountID && x.RoleID == a.RoleID && x.PadPattern == a.PadPattern {
			return ErrConflict
		}
	}
	m.acc.assignments = append(m.acc.assignments, a)
	return nil
}

func (m *MemoryDataStore) RemoveRoleAssignment(accountID, roleID, padPattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, x := range m.acc.assignments {
		if x.AccountID == accountID && x.RoleID == roleID && x.PadPattern == padPattern {
			m.acc.assignments = append(m.acc.assignments[:i], m.acc.assignments[i+1:]...)
			return nil
		}
	}
	return ErrAssignmentNotFound
}

func (m *MemoryDataStore) ListRoleAssignments(accountID string) ([]RoleAssignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []RoleAssignment{}
	for _, a := range m.acc.assignments {
		if accountID == "" || a.AccountID == accountID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *MemoryDataStore) AppendAudit(e AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acc.audit = append(m.acc.audit, e)
	return nil
}

func (m *MemoryDataStore) ListAudit(limit int) ([]AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []AuditEntry{}
	for i := len(m.acc.audit) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.acc.audit[i])
	}
	return out, nil
}

// --- AccountMethods of the SQL stores ---

func (d MysqlDB) accounts() sqlAccounts   { return sqlAccounts{db: d.sqlDB, begin: d.sqlDB.Begin} }
func (d SQLiteDB) accounts() sqlAccounts  { return sqlAccounts{db: d.sqlDB, begin: d.sqlDB.Begin} }
func (d PostgresDB) accounts() pgAccounts { return pgAccounts{d: d} }

func (d MysqlDB) CreateAccount(a Account) error          { return d.accounts().CreateAccount(a) }
func (d MysqlDB) GetAccount(id string) (*Account, error) { return d.accounts().GetAccount(id) }
func (d MysqlDB) GetAccountByUsername(u string) (*Account, error) {
	return d.accounts().GetAccountByUsername(u)
}
func (d MysqlDB) ListAccounts() ([]Account, error)         { return d.accounts().ListAccounts() }
func (d MysqlDB) UpdateAccount(a Account) error            { return d.accounts().UpdateAccount(a) }
func (d MysqlDB) DeleteAccount(id string) error            { return d.accounts().DeleteAccount(id) }
func (d MysqlDB) CreateRole(r Role) error                  { return d.accounts().CreateRole(r) }
func (d MysqlDB) GetRole(id string) (*Role, error)         { return d.accounts().GetRole(id) }
func (d MysqlDB) ListRoles() ([]Role, error)               { return d.accounts().ListRoles() }
func (d MysqlDB) UpdateRole(r Role) error                  { return d.accounts().UpdateRole(r) }
func (d MysqlDB) DeleteRole(id string) error               { return d.accounts().DeleteRole(id) }
func (d MysqlDB) AddRoleAssignment(a RoleAssignment) error { return d.accounts().AddRoleAssignment(a) }
func (d MysqlDB) RemoveRoleAssignment(accountID, roleID, padPattern string) error {
	return d.accounts().RemoveRoleAssignment(accountID, roleID, padPattern)
}
func (d MysqlDB) ListRoleAssignments(accountID string) ([]RoleAssignment, error) {
	return d.accounts().ListRoleAssignments(accountID)
}
func (d MysqlDB) AppendAudit(e AuditEntry) error            { return d.accounts().AppendAudit(e) }
func (d MysqlDB) ListAudit(limit int) ([]AuditEntry, error) { return d.accounts().ListAudit(limit) }

func (d SQLiteDB) CreateAccount(a Account) error          { return d.accounts().CreateAccount(a) }
func (d SQLiteDB) GetAccount(id string) (*Account, error) { return d.accounts().GetAccount(id) }
func (d SQLiteDB) GetAccountByUsername(u string) (*Account, error) {
	return d.accounts().GetAccountByUsername(u)
}
func (d SQLiteDB) ListAccounts() ([]Account, error)         { return d.accounts().ListAccounts() }
func (d SQLiteDB) UpdateAccount(a Account) error            { return d.accounts().UpdateAccount(a) }
func (d SQLiteDB) DeleteAccount(id string) error            { return d.accounts().DeleteAccount(id) }
func (d SQLiteDB) CreateRole(r Role) error                  { return d.accounts().CreateRole(r) }
func (d SQLiteDB) GetRole(id string) (*Role, error)         { return d.accounts().GetRole(id) }
func (d SQLiteDB) ListRoles() ([]Role, error)               { return d.accounts().ListRoles() }
func (d SQLiteDB) UpdateRole(r Role) error                  { return d.accounts().UpdateRole(r) }
func (d SQLiteDB) DeleteRole(id string) error               { return d.accounts().DeleteRole(id) }
func (d SQLiteDB) AddRoleAssignment(a RoleAssignment) error { return d.accounts().AddRoleAssignment(a) }
func (d SQLiteDB) RemoveRoleAssignment(accountID, roleID, padPattern string) error {
	return d.accounts().RemoveRoleAssignment(accountID, roleID, padPattern)
}
func (d SQLiteDB) ListRoleAssignments(accountID string) ([]RoleAssignment, error) {
	return d.accounts().ListRoleAssignments(accountID)
}
func (d SQLiteDB) AppendAudit(e AuditEntry) error            { return d.accounts().AppendAudit(e) }
func (d SQLiteDB) ListAudit(limit int) ([]AuditEntry, error) { return d.accounts().ListAudit(limit) }

func (d PostgresDB) CreateAccount(a Account) error          { return d.accounts().CreateAccount(a) }
func (d PostgresDB) GetAccount(id string) (*Account, error) { return d.accounts().GetAccount(id) }
func (d PostgresDB) GetAccountByUsername(u string) (*Account, error) {
	return d.accounts().GetAccountByUsername(u)
}
func (d PostgresDB) ListAccounts() ([]Account, error) { return d.accounts().ListAccounts() }
func (d PostgresDB) UpdateAccount(a Account) error    { return d.accounts().UpdateAccount(a) }
func (d PostgresDB) DeleteAccount(id string) error    { return d.accounts().DeleteAccount(id) }
func (d PostgresDB) CreateRole(r Role) error          { return d.accounts().CreateRole(r) }
func (d PostgresDB) GetRole(id string) (*Role, error) { return d.accounts().GetRole(id) }
func (d PostgresDB) ListRoles() ([]Role, error)       { return d.accounts().ListRoles() }
func (d PostgresDB) UpdateRole(r Role) error          { return d.accounts().UpdateRole(r) }
func (d PostgresDB) DeleteRole(id string) error       { return d.accounts().DeleteRole(id) }
func (d PostgresDB) AddRoleAssignment(a RoleAssignment) error {
	return d.accounts().AddRoleAssignment(a)
}
func (d PostgresDB) RemoveRoleAssignment(accountID, roleID, padPattern string) error {
	return d.accounts().RemoveRoleAssignment(accountID, roleID, padPattern)
}
func (d PostgresDB) ListRoleAssignments(accountID string) ([]RoleAssignment, error) {
	return d.accounts().ListRoleAssignments(accountID)
}
func (d PostgresDB) AppendAudit(e AuditEntry) error            { return d.accounts().AppendAudit(e) }
func (d PostgresDB) ListAudit(limit int) ([]AuditEntry, error) { return d.accounts().ListAudit(limit) }

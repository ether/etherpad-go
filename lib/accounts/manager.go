package accounts

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
)

// CacheTTL is how long role definitions and resolved grants are cached. A
// change on another instance of a cluster is seen after at most this long.
const CacheTTL = 30 * time.Second

// Login throttling: after maxFailures failed logins within failureWindow
// an account is locked for failureWindow.
const (
	maxFailures   = 5
	failureWindow = 15 * time.Minute
	// verifiedFor is how long a checked password is remembered, so HTTP
	// basic auth on every request does not hash on every request.
	verifiedFor = time.Minute
)

// MaxUsernameLength is the longest username.
const MaxUsernameLength = 64

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrLocked             = errors.New("too many failed logins, try again later")
	ErrForbidden          = errors.New("not allowed")
	ErrLastAdmin          = errors.New("the last active admin cannot lose admin rights")
	ErrAdminRoleFixed     = errors.New("the admin role cannot be changed or deleted")
	ErrBuiltinRole        = errors.New("built-in roles cannot be deleted")
	ErrAnonymousScope     = errors.New("visitors cannot get admin scopes, tokens:own or ai:unlimited, logged-in people as a whole no admin scopes")
	ErrRoleCycle          = errors.New("a role cannot include itself")
	ErrEscalation         = errors.New("cannot grant scopes you do not have yourself")
	ErrInvalidUsername    = errors.New("invalid username")
	ErrInvalidRole        = errors.New("invalid role")
	ErrNoPassword         = errors.New("service and anonymous accounts have no password")
	ErrAnonymousAccount   = errors.New("the anonymous account cannot be changed this way")
)

// Actor is who makes a change: an account id and its grants. System is the
// server itself (seeding, migration).
type Actor struct {
	ID     string
	Grants Grants
}

// System acts for the server: it may do everything.
var System = Actor{ID: "system", Grants: Grants{grants: []grant{{all: true}}}}

type cachedGrants struct {
	grants Grants
	at     time.Time
}

type verified struct {
	accountID string
	until     time.Time
}

// Manager reads and changes accounts, roles and assignments and enforces
// the rules on them.
type Manager struct {
	store db.AccountMethods
	now   func() time.Time

	mu       sync.Mutex
	roles    map[string]db.Role
	rolesAt  time.Time
	grants   map[string]cachedGrants
	verified map[string]verified
	failures map[string][]time.Time
	authKey  []byte

	seedMu sync.Mutex
	seeded bool
}

// NewManager returns a Manager on store.
func NewManager(store db.AccountMethods) *Manager {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return &Manager{
		store:    store,
		now:      time.Now,
		grants:   map[string]cachedGrants{},
		verified: map[string]verified{},
		failures: map[string][]time.Time{},
		authKey:  key,
	}
}

// Invalidate forgets the cached roles and grants, e.g. after another
// instance changed them.
func (m *Manager) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles = nil
	m.grants = map[string]cachedGrants{}
	m.verified = map[string]verified{}
}

// Roles returns the role definitions by id.
func (m *Manager) Roles() (map[string]db.Role, error) {
	m.mu.Lock()
	if m.roles != nil && m.now().Sub(m.rolesAt) < CacheTTL {
		roles := m.roles
		m.mu.Unlock()
		return roles, nil
	}
	m.mu.Unlock()
	list, err := m.store.ListRoles()
	if err != nil {
		return nil, err
	}
	roles := make(map[string]db.Role, len(list))
	for _, r := range list {
		roles[r.ID] = r
	}
	m.mu.Lock()
	m.roles, m.rolesAt = roles, m.now()
	m.mu.Unlock()
	return roles, nil
}

// Refs returns the assignments of an account as role references.
func (m *Manager) Refs(accountID string) ([]RoleRef, error) {
	as, err := m.store.ListRoleAssignments(accountID)
	if err != nil {
		return nil, err
	}
	refs := make([]RoleRef, len(as))
	for i, a := range as {
		refs[i] = RoleRef{Role: a.RoleID, Pads: a.PadPattern}
	}
	return refs, nil
}

// GrantsFor returns what the account may do: its own assignments, those of
// the authenticated account for people and those of the anonymous account
// (logged in, nobody has less than a visitor).
func (m *Manager) GrantsFor(accountID string) (Grants, error) {
	m.mu.Lock()
	if c, ok := m.grants[accountID]; ok && m.now().Sub(c.at) < CacheTTL {
		m.mu.Unlock()
		return c.grants, nil
	}
	m.mu.Unlock()
	refs, err := m.Refs(accountID)
	if err != nil {
		return Grants{}, err
	}
	for _, pseudo := range pseudoAccountsOf(accountID) {
		more, err := m.Refs(pseudo)
		if err != nil {
			return Grants{}, err
		}
		refs = append(refs, more...)
	}
	roles, err := m.Roles()
	if err != nil {
		return Grants{}, err
	}
	g := resolve(roles, refs)
	m.mu.Lock()
	m.grants[accountID] = cachedGrants{grants: g, at: m.now()}
	m.mu.Unlock()
	return g, nil
}

// AnonymousGrants returns what visitors who are not logged in may do.
func (m *Manager) AnonymousGrants() (Grants, error) { return m.GrantsFor(AnonymousID) }

// AuthenticatedGrants returns what a logged-in person without an account
// of its own (authenticated by a plugin) may do.
func (m *Manager) AuthenticatedGrants() (Grants, error) { return m.GrantsFor(AuthenticatedID) }

// GrantsFromRefs resolves the assignments a verified token of a person
// carries, with those of the authenticated and anonymous accounts added.
func (m *Manager) GrantsFromRefs(refs []RoleRef) (Grants, error) {
	all := slices.Clone(refs)
	for _, pseudo := range []string{AuthenticatedID, AnonymousID} {
		more, err := m.Refs(pseudo)
		if err != nil {
			return Grants{}, err
		}
		all = append(all, more...)
	}
	roles, err := m.Roles()
	if err != nil {
		return Grants{}, err
	}
	return resolve(roles, all), nil
}

// pseudoAccountsOf lists the built-in accounts whose roles accountID gets
// too.
func pseudoAccountsOf(accountID string) []string {
	switch accountID {
	case AnonymousID:
		return nil
	case AuthenticatedID:
		return []string{AnonymousID}
	}
	return []string{AuthenticatedID, AnonymousID}
}

// Account returns the account, or nil.
func (m *Manager) Account(id string) (*db.Account, error) { return m.store.GetAccount(id) }

// AccountByUsername returns the account, or nil.
func (m *Manager) AccountByUsername(username string) (*db.Account, error) {
	return m.store.GetAccountByUsername(username)
}

// Accounts returns all accounts, oldest first.
func (m *Manager) Accounts() ([]db.Account, error) { return m.store.ListAccounts() }

func (m *Manager) verifiedKey(username, password string) string {
	mac := hmac.New(sha256.New, m.authKey)
	mac.Write([]byte(username))
	mac.Write([]byte{0})
	mac.Write([]byte(password))
	return hex.EncodeToString(mac.Sum(nil))
}

// Authenticate checks a username and password. Disabled accounts, service
// accounts and the anonymous account cannot log in. After repeated
// failures the username is locked for a while (ErrLocked).
func (m *Manager) Authenticate(username, password string) (*db.Account, error) {
	now := m.now()
	key := m.verifiedKey(username, password)
	m.mu.Lock()
	if v, ok := m.verified[key]; ok && now.Before(v.until) {
		m.mu.Unlock()
		a, err := m.store.GetAccount(v.accountID)
		if err != nil {
			return nil, err
		}
		if a != nil && a.DisabledAt == nil && a.Username == username {
			return a, nil
		}
		return nil, ErrInvalidCredentials
	}
	recent := m.failures[username][:0]
	for _, t := range m.failures[username] {
		if now.Sub(t) < failureWindow {
			recent = append(recent, t)
		}
	}
	m.failures[username] = recent
	locked := len(recent) >= maxFailures
	m.mu.Unlock()
	if locked {
		return nil, ErrLocked
	}

	a, err := m.store.GetAccountByUsername(username)
	if err != nil {
		return nil, err
	}
	ok := a != nil && a.Kind == KindHuman && a.DisabledAt == nil && a.PasswordHash != "" && CheckPassword(a.PasswordHash, password)
	if !ok {
		if a == nil {
			// Take as long as a real check, so usernames cannot be told
			// apart by timing.
			CheckPassword(dummyHash, password)
		}
		m.mu.Lock()
		m.failures[username] = append(m.failures[username], now)
		m.mu.Unlock()
		return nil, ErrInvalidCredentials
	}
	m.mu.Lock()
	delete(m.failures, username)
	m.verified[key] = verified{accountID: a.ID, until: now.Add(verifiedFor)}
	m.mu.Unlock()
	login := now
	a.LastLoginAt = &login
	_ = m.store.UpdateAccount(*a)
	return a, nil
}

var dummyHash, _ = HashPassword("not a password of anyone")

func (m *Manager) audit(actor Actor, action, target string, details any) {
	b, _ := json.Marshal(details)
	_ = m.store.AppendAudit(db.AuditEntry{
		ID: utils.RandomString(16), At: m.now(), ActorID: actor.ID, Action: action, Target: target, Details: string(b),
	})
}

// Audit returns the newest audit entries first.
func (m *Manager) Audit(limit int) ([]db.AuditEntry, error) { return m.store.ListAudit(limit) }

func validUsername(u string) bool {
	if u == "" || utf8.RuneCountInString(u) > MaxUsernameLength || strings.TrimSpace(u) != u {
		return false
	}
	return !strings.ContainsAny(u, ":\x00\n\r\t")
}

// NewAccount describes an account to create.
type NewAccount struct {
	Username    string
	DisplayName string
	Email       string
	Kind        string
	// Password, only for people; "" leaves the account without one.
	Password string
	// PasswordHash takes over an existing hash instead of Password.
	PasswordHash       string
	MustChangePassword bool
}

// CreateAccount creates a person's or a service account.
func (m *Manager) CreateAccount(actor Actor, n NewAccount) (*db.Account, error) {
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return nil, ErrForbidden
	}
	if !validUsername(n.Username) {
		return nil, ErrInvalidUsername
	}
	if n.Kind != KindHuman && n.Kind != KindService {
		return nil, fmt.Errorf("unknown account kind %q", n.Kind)
	}
	hash := n.PasswordHash
	if n.Password != "" {
		if utf8.RuneCountInString(n.Password) < MinPasswordLength {
			return nil, ErrPasswordTooShort
		}
		var err error
		if hash, err = HashPassword(n.Password); err != nil {
			return nil, err
		}
	}
	if n.Kind == KindService && hash != "" {
		return nil, ErrNoPassword
	}
	display := n.DisplayName
	if display == "" {
		display = n.Username
	}
	a := db.Account{
		ID: "acc." + utils.RandomString(16), Username: n.Username, DisplayName: display, Email: n.Email,
		Kind: n.Kind, PasswordHash: hash, MustChangePassword: n.MustChangePassword, CreatedAt: m.now(),
	}
	if err := m.store.CreateAccount(a); err != nil {
		return nil, err
	}
	m.audit(actor, "account.create", a.ID, map[string]string{"username": a.Username, "kind": a.Kind})
	return &a, nil
}

// SetPassword sets a person's password; actor is the person or an admin.
func (m *Manager) SetPassword(actor Actor, accountID, password string, mustChange bool) error {
	if actor.ID != accountID && !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	a, err := m.store.GetAccount(accountID)
	if err != nil {
		return err
	}
	if a == nil {
		return db.ErrAccountNotFound
	}
	if a.Kind != KindHuman {
		return ErrNoPassword
	}
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if a.PasswordHash, err = HashPassword(password); err != nil {
		return err
	}
	a.MustChangePassword = mustChange
	if err := m.store.UpdateAccount(*a); err != nil {
		return err
	}
	m.Invalidate()
	m.audit(actor, "account.password", a.ID, nil)
	return nil
}

// isAdminAccount reports whether the account holds the admin role's rights
// for all pads.
func (m *Manager) isAdminAccount(roles map[string]db.Role, accountID string, without *db.RoleAssignment) (bool, error) {
	as, err := m.store.ListRoleAssignments(accountID)
	if err != nil {
		return false, err
	}
	for _, a := range as {
		if a.PadPattern != "" || (without != nil && *without == a) {
			continue
		}
		if _, all := closureScopes(roles, a.RoleID); all {
			return true, nil
		}
	}
	return false, nil
}

// otherActiveAdmin reports whether an active person other than accountID
// is an admin.
func (m *Manager) otherActiveAdmin(roles map[string]db.Role, accountID string) (bool, error) {
	accounts, err := m.store.ListAccounts()
	if err != nil {
		return false, err
	}
	for _, a := range accounts {
		if a.ID == accountID || a.Kind != KindHuman || a.DisabledAt != nil {
			continue
		}
		if ok, err := m.isAdminAccount(roles, a.ID, nil); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// guardLastAdmin refuses a change that would leave no active admin: the
// account would stop being an admin (without, when only an assignment is
// removed).
func (m *Manager) guardLastAdmin(accountID string, without *db.RoleAssignment) error {
	roles, err := m.Roles()
	if err != nil {
		return err
	}
	admin, err := m.isAdminAccount(roles, accountID, nil)
	if err != nil || !admin {
		return err
	}
	if without != nil {
		if still, err := m.isAdminAccount(roles, accountID, without); err != nil || still {
			return err
		}
	}
	other, err := m.otherActiveAdmin(roles, accountID)
	if err != nil {
		return err
	}
	if !other {
		return ErrLastAdmin
	}
	return nil
}

// SetDisabled disables or enables an account.
func (m *Manager) SetDisabled(actor Actor, accountID string, disabled bool) error {
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	if isPseudo(accountID) {
		return ErrAnonymousAccount
	}
	a, err := m.store.GetAccount(accountID)
	if err != nil {
		return err
	}
	if a == nil {
		return db.ErrAccountNotFound
	}
	if disabled {
		if err := m.guardLastAdmin(accountID, nil); err != nil {
			return err
		}
		now := m.now()
		a.DisabledAt = &now
	} else {
		a.DisabledAt = nil
	}
	if err := m.store.UpdateAccount(*a); err != nil {
		return err
	}
	m.Invalidate()
	m.audit(actor, map[bool]string{true: "account.disable", false: "account.enable"}[disabled], a.ID, nil)
	return nil
}

// DeleteAccount removes an account and its assignments.
func (m *Manager) DeleteAccount(actor Actor, accountID string) error {
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	if isPseudo(accountID) {
		return ErrAnonymousAccount
	}
	if err := m.guardLastAdmin(accountID, nil); err != nil {
		return err
	}
	if err := m.store.DeleteAccount(accountID); err != nil {
		return err
	}
	m.Invalidate()
	m.audit(actor, "account.delete", accountID, nil)
	return nil
}

// allows reports whether the actor has every scope in set (for all pads):
// nobody grants what they do not have.
func allows(actor Grants, set map[string]bool, all bool) bool {
	if all {
		return actor.isAll()
	}
	for s := range set {
		if !actor.HasGlobal(s) {
			return false
		}
	}
	return true
}

func (g Grants) isAll() bool {
	for _, gr := range g.grants {
		if gr.all && gr.pattern == "" {
			return true
		}
	}
	return false
}

// pseudoOK reports whether the anonymous and authenticated accounts keep
// within what they may have, with roles as the role definitions and extra
// as an assignment about to be added.
func (m *Manager) pseudoOK(roles map[string]db.Role, extra *db.RoleAssignment) (bool, error) {
	for _, id := range []string{AnonymousID, AuthenticatedID} {
		as, err := m.store.ListRoleAssignments(id)
		if err != nil {
			return false, err
		}
		if extra != nil && extra.AccountID == id {
			as = append(as, *extra)
		}
		forbidden := anonymousForbidden
		if id == AuthenticatedID {
			forbidden = IsAdminScope
		}
		for _, a := range as {
			set, all := closureScopes(roles, a.RoleID)
			if all {
				return false, nil
			}
			for s := range set {
				if forbidden(s) {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

func hasCycle(roles map[string]db.Role, start string) bool {
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(id string) bool {
		for _, inc := range roles[id].Includes {
			if inc == start {
				return true
			}
			if seen[inc] {
				continue
			}
			seen[inc] = true
			if walk(inc) {
				return true
			}
		}
		return false
	}
	return walk(start)
}

// SaveRole creates the role (no ID yet, or an unknown one) or changes it.
func (m *Manager) SaveRole(actor Actor, r db.Role) (*db.Role, error) {
	if !actor.Grants.HasGlobal(ScopeAdminRoles) {
		return nil, ErrForbidden
	}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || utf8.RuneCountInString(r.Name) > MaxUsernameLength {
		return nil, ErrInvalidRole
	}
	if r.ID == RoleAdmin {
		return nil, ErrAdminRoleFixed
	}
	roles, err := m.Roles()
	if err != nil {
		return nil, err
	}
	existing, known := roles[r.ID]
	if r.ID == "" {
		r.ID = "role." + utils.RandomString(12)
	}
	r.Builtin = known && existing.Builtin
	r.Scopes = uniqueSorted(r.Scopes)
	r.Includes = uniqueSorted(r.Includes)
	for _, inc := range r.Includes {
		if _, ok := roles[inc]; !ok || inc == r.ID {
			return nil, ErrRoleCycle
		}
	}
	next := make(map[string]db.Role, len(roles)+1)
	for k, v := range roles {
		next[k] = v
	}
	next[r.ID] = r
	if hasCycle(next, r.ID) {
		return nil, ErrRoleCycle
	}
	// What the role gives after the change must be the actor's to give;
	// so must what it gave before (nobody takes rights from a role they
	// could not have handed out).
	set, all := closureScopes(next, r.ID)
	if !allows(actor.Grants, set, all) {
		return nil, ErrEscalation
	}
	if known {
		oldSet, oldAll := closureScopes(roles, r.ID)
		if !allows(actor.Grants, oldSet, oldAll) {
			return nil, ErrEscalation
		}
	}
	if ok, err := m.pseudoOK(next, nil); err != nil || !ok {
		if err == nil {
			err = ErrAnonymousScope
		}
		return nil, err
	}
	if known {
		err = m.store.UpdateRole(r)
	} else {
		err = m.store.CreateRole(r)
	}
	if err != nil {
		return nil, err
	}
	m.Invalidate()
	m.audit(actor, map[bool]string{true: "role.update", false: "role.create"}[known], r.ID,
		map[string]any{"name": r.Name, "scopes": r.Scopes, "includes": r.Includes, "limits": r.Limits})
	return &r, nil
}

func uniqueSorted(s []string) []string {
	set := map[string]bool{}
	for _, x := range s {
		if x = strings.TrimSpace(x); x != "" {
			set[x] = true
		}
	}
	return sortedKeys(set)
}

// DeleteRole removes a role that is not built in.
func (m *Manager) DeleteRole(actor Actor, id string) error {
	if !actor.Grants.HasGlobal(ScopeAdminRoles) {
		return ErrForbidden
	}
	if id == RoleAdmin {
		return ErrAdminRoleFixed
	}
	roles, err := m.Roles()
	if err != nil {
		return err
	}
	r, ok := roles[id]
	if !ok {
		return db.ErrRoleNotFound
	}
	if r.Builtin {
		return ErrBuiltinRole
	}
	set, all := closureScopes(roles, id)
	if !allows(actor.Grants, set, all) {
		return ErrEscalation
	}
	// A role that makes someone an admin (by including admin) cannot take
	// the last admin with it.
	if all {
		accounts, err := m.store.ListAccounts()
		if err != nil {
			return err
		}
		for _, a := range accounts {
			as, _ := m.store.ListRoleAssignments(a.ID)
			for _, as := range as {
				if as.RoleID == id && as.PadPattern == "" {
					if err := m.guardLastAdmin(a.ID, &as); err != nil {
						return err
					}
				}
			}
		}
	}
	if err := m.store.DeleteRole(id); err != nil {
		return err
	}
	m.Invalidate()
	m.audit(actor, "role.delete", id, map[string]string{"name": r.Name})
	return nil
}

// Assign gives an account a role for the pads matching padPattern ("" or
// "*": all pads).
func (m *Manager) Assign(actor Actor, accountID, roleID, padPattern string) error {
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	if padPattern == "*" {
		padPattern = ""
	}
	if err := ValidatePattern(padPattern); err != nil {
		return err
	}
	a, err := m.store.GetAccount(accountID)
	if err != nil {
		return err
	}
	if a == nil {
		return db.ErrAccountNotFound
	}
	roles, err := m.Roles()
	if err != nil {
		return err
	}
	if _, ok := roles[roleID]; !ok {
		return db.ErrRoleNotFound
	}
	set, all := closureScopes(roles, roleID)
	if !allows(actor.Grants, set, all) {
		return ErrEscalation
	}
	as := db.RoleAssignment{AccountID: accountID, RoleID: roleID, PadPattern: padPattern, CreatedBy: actor.ID, CreatedAt: m.now()}
	if isPseudo(accountID) {
		if ok, err := m.pseudoOK(roles, &as); err != nil || !ok {
			if err == nil {
				err = ErrAnonymousScope
			}
			return err
		}
	}
	if err := m.store.AddRoleAssignment(as); err != nil {
		return err
	}
	m.Invalidate()
	m.audit(actor, "assignment.add", accountID, map[string]string{"role": roleID, "pads": padPattern})
	return nil
}

// Unassign takes a role assignment away.
func (m *Manager) Unassign(actor Actor, accountID, roleID, padPattern string) error {
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	if padPattern == "*" {
		padPattern = ""
	}
	roles, err := m.Roles()
	if err != nil {
		return err
	}
	set, all := closureScopes(roles, roleID)
	if !allows(actor.Grants, set, all) {
		return ErrEscalation
	}
	as, err := m.store.ListRoleAssignments(accountID)
	if err != nil {
		return err
	}
	for _, x := range as {
		if x.RoleID == roleID && x.PadPattern == padPattern {
			if err := m.guardLastAdmin(accountID, &x); err != nil {
				return err
			}
			break
		}
	}
	if err := m.store.RemoveRoleAssignment(accountID, roleID, padPattern); err != nil {
		return err
	}
	m.Invalidate()
	m.audit(actor, "assignment.remove", accountID, map[string]string{"role": roleID, "pads": padPattern})
	return nil
}

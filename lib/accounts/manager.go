package accounts

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"time"

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
	ErrInvalidProfile     = errors.New("invalid display name or e-mail")
	ErrNotBuiltin         = errors.New("only built-in roles can be reset")
)

// Actor is who makes a change: an account id and its grants. System is the
// server itself (seeding, migration).
type Actor struct {
	ID     string
	Grants Grants
	// server marks System and CLI, which are not accounts.
	server bool
}

// System acts for the server: it may do everything.
var System = Actor{ID: "system", Grants: Grants{grants: []grant{{all: true}}}, server: true}

// CLI acts for the command line (etherpad-go admin): it may do everything;
// the audit log names it.
var CLI = Actor{ID: "cli", Grants: Grants{grants: []grant{{all: true}}}, server: true}

type cachedGrants struct {
	grants Grants
	at     time.Time
}

type verified struct {
	accountID string
	// hash is the password hash checked: a new password (also set by
	// another process) ends the remembered login.
	hash  string
	until time.Time
}

// Manager reads and changes accounts, roles and assignments and enforces
// the rules on them.
type Manager struct {
	store db.AccountMethods
	now   func() time.Time

	mu      sync.Mutex
	roles   map[string]db.Role
	rolesAt time.Time
	grants  map[string]cachedGrants
	people  map[string]cachedPerson
	holders map[string]cachedPerson
	// gen counts the times the caches were forgotten, so a read that
	// began before does not cache what it found (Person).
	gen      uint64
	verified map[string]verified
	throttle map[string]*throttle
	authKey  []byte

	// wmu serializes the changes of this process; with the database lock
	// of lockWrites the changes of all instances.
	wmu      sync.Mutex
	onChange func()

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
		people:   map[string]cachedPerson{},
		holders:  map[string]cachedPerson{},
		verified: map[string]verified{},
		throttle: map[string]*throttle{},
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
	m.people = map[string]cachedPerson{}
	m.holders = map[string]cachedPerson{}
	m.gen++
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
// the authenticated account for people (not for service accounts, which
// never log in) and those of the anonymous account (nobody has less than a
// visitor).
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
	pseudos := pseudoAccountsOf(accountID)
	if !isPseudo(accountID) {
		// A service account never logs in: it is not one of everyone
		// logged in, only what its own roles say (and what visitors may).
		a, err := m.store.GetAccount(accountID)
		if err != nil && !errors.Is(err, db.ErrAccountNotFound) {
			return Grants{}, err
		}
		if a != nil && a.Kind == KindService {
			pseudos = []string{AnonymousID}
		}
	}
	for _, pseudo := range pseudos {
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
// accounts and the built-in accounts cannot log in. After repeated failures
// the username is locked for a while (ErrLocked); attempts running at the
// same time count against the limit before they are checked.
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
		if a != nil && a.DisabledAt == nil && a.Username == username && a.PasswordHash == v.hash {
			return a, nil
		}
		return nil, ErrInvalidCredentials
	}
	t := m.throttleLocked(username, now)
	if len(t.failures)+t.inFlight >= maxFailures {
		m.mu.Unlock()
		return nil, ErrLocked
	}
	t.inFlight++
	m.mu.Unlock()

	ok, a, err := m.checkCredentials(username, password)

	m.mu.Lock()
	t.inFlight--
	if err == nil && !ok {
		t.failures = append(t.failures, now)
	}
	if ok {
		delete(m.throttle, username)
		m.verified[key] = verified{accountID: a.ID, hash: a.PasswordHash, until: now.Add(verifiedFor)}
	}
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}
	login := now
	a.LastLoginAt = &login
	// Only the login time: writing the whole row read before the check
	// could undo a change made meanwhile (a disable, a new password).
	_ = m.store.TouchLogin(a.ID, login)
	return a, nil
}

// throttle is the login state of a username.
type throttle struct {
	failures []time.Time
	inFlight int
}

// maxThrottled bounds the usernames whose failures are remembered: logins
// with ever new usernames cannot fill the memory.
const maxThrottled = 10000

// throttleLocked returns the throttle state of username with old failures
// dropped. The caller holds m.mu.
func (m *Manager) throttleLocked(username string, now time.Time) *throttle {
	if len(m.throttle) >= maxThrottled {
		for name, t := range m.throttle {
			t.failures = recentFailures(t.failures, now)
			if len(t.failures) == 0 && t.inFlight == 0 {
				delete(m.throttle, name)
			}
		}
		if len(m.throttle) >= maxThrottled {
			// Under a flood: forget the locks rather than the memory.
			m.throttle = map[string]*throttle{}
		}
	}
	t := m.throttle[username]
	if t == nil {
		t = &throttle{}
		m.throttle[username] = t
	}
	t.failures = recentFailures(t.failures, now)
	return t
}

func recentFailures(failures []time.Time, now time.Time) []time.Time {
	kept := failures[:0]
	for _, f := range failures {
		if now.Sub(f) < failureWindow {
			kept = append(kept, f)
		}
	}
	return kept
}

// hashSlots bounds the password checks running at once: each takes 19 MiB.
var hashSlots = make(chan struct{}, max(2, min(8, runtime.NumCPU())))

// checkCredentials checks the password of a person who may log in.
func (m *Manager) checkCredentials(username, password string) (bool, *db.Account, error) {
	a, err := m.store.GetAccountByUsername(username)
	if err != nil {
		return false, nil, err
	}
	hashSlots <- struct{}{}
	defer func() { <-hashSlots }()
	if a == nil || a.Kind != KindHuman || a.DisabledAt != nil || a.PasswordHash == "" {
		// Take as long as a real check, so usernames cannot be told
		// apart by timing.
		CheckPassword(dummyHash, password)
		return false, nil, nil
	}
	return CheckPassword(a.PasswordHash, password), a, nil
}

// PasswordMatches reports whether password is the account's current one,
// without counting as a login attempt.
func (m *Manager) PasswordMatches(a *db.Account, password string) bool {
	if a == nil || a.PasswordHash == "" {
		return false
	}
	hashSlots <- struct{}{}
	defer func() { <-hashSlots }()
	return CheckPassword(a.PasswordHash, password)
}

var dummyHash, _ = HashPassword("not a password of anyone")

// ErrAuditFailed is returned when a change was saved but its audit entry
// could not be written.
var ErrAuditFailed = errors.New("the change was saved, but the audit log could not be written")

func (m *Manager) audit(actor Actor, action, target string, details any) error {
	b, _ := json.Marshal(details)
	if err := m.store.AppendAudit(db.AuditEntry{
		ID: utils.RandomString(16), At: m.now(), ActorID: actor.ID, Action: action, Target: target, Details: string(b),
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrAuditFailed, err)
	}
	return nil
}

// Audit returns the newest audit entries first.
func (m *Manager) Audit(limit int) ([]db.AuditEntry, error) { return m.store.ListAudit(limit) }

// Assignments returns all role assignments.
func (m *Manager) Assignments() ([]db.RoleAssignment, error) { return m.store.ListRoleAssignments("") }

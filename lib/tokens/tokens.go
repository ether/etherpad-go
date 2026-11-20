// Package tokens manages the tokens of accounts: API keys with their own
// scopes, pad pattern and expiry. A token never has more rights than its
// account: what it may do is the account's rights, cut down to its scopes
// and pattern, evaluated on every use (accounts.Grants.Restrict). Only a
// SHA-256 hash of a key is stored; the key is shown once.
package tokens

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
)

const (
	// KeyPrefix starts every key, so leaked keys are easy to find.
	KeyPrefix = "ep_"
	keyLength = 40
	// shownPrefix is how much of a key lists show.
	shownPrefix = 10
	// MaxName is the longest token name, in characters.
	MaxName = 100
	// touchEvery is how often the last use of a token is written.
	touchEvery = time.Minute
	// cacheTTL is how long a looked-up token is trusted without reading it
	// again; every change forgets it at once, also on the other instances.
	cacheTTL = 30 * time.Second
	maxCached = 10000
)

var (
	ErrNoName         = errors.New("a name is required")
	ErrNameTooLong    = errors.New("the name is too long")
	ErrUnknownScope   = errors.New("unknown scope")
	ErrNoScopes       = errors.New("a token needs at least one scope")
	ErrForbidden      = errors.New("you may not manage tokens of this account")
	ErrEscalation     = errors.New("you cannot give a token rights you do not have")
	ErrNoAccount      = errors.New("the account does not exist or is disabled")
	ErrExpiryRequired = errors.New("tokens must expire on this server")
	ErrExpiryTooLate  = errors.New("the expiry is later than this server allows")
	ErrExpiryPast     = errors.New("the expiry is in the past")
	ErrNotActive      = errors.New("the token is revoked or expired")
	ErrInvalidLimit   = errors.New("a limit cannot be negative")
	ErrInvalidPattern = errors.New("invalid pad pattern")
	ErrTokenNotFound  = db.ErrTokenNotFound
	// ErrInvalid is any failure to use a key; the reason is not told.
	ErrInvalid = errors.New("invalid token")
)

// Hash is the stored form of a key (the same as the former API keys', so
// their keys keep working as tokens).
func Hash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Store is what the manager needs of the database.
type Store interface {
	db.TokenMethods
}

// Manager issues, rotates, revokes and checks tokens.
type Manager struct {
	store    Store
	accounts *accounts.Manager
	policy   func() settings.Tokens
	now      func() time.Time

	mu      sync.Mutex
	cache   map[string]cachedToken
	touched map[string]time.Time
	// wmu serializes the changes of this process.
	wmu sync.Mutex
}

type cachedToken struct {
	token *db.Token
	at    time.Time
	gen   uint64
}

// NewManager returns a manager on store, with the accounts' manager and the
// expiry policy (read on every change, so a changed setting counts).
func NewManager(store Store, acc *accounts.Manager, policy func() settings.Tokens) *Manager {
	if policy == nil {
		policy = func() settings.Tokens { return settings.Tokens{} }
	}
	return &Manager{store: store, accounts: acc, policy: policy, now: time.Now,
		cache: map[string]cachedToken{}, touched: map[string]time.Time{}}
}

var (
	registryMu sync.Mutex
	registry   = map[db.DataStore]*Manager{}
)

// For returns the manager of a data store, made once.
func For(store db.DataStore, cfg *settings.Settings) *Manager {
	registryMu.Lock()
	defer registryMu.Unlock()
	if m, ok := registry[store]; ok {
		return m
	}
	m := NewManager(store, accounts.For(store), func() settings.Tokens { return cfg.Tokens })
	registry[store] = m
	return m
}

// Spec is a token to issue: for which account, what it may do, how long.
type Spec struct {
	AccountID string
	Name      string
	// Scopes the token may use; accounts.AllScopes for all of them.
	Scopes []string
	// PadPattern limits it to matching pads; "" for all.
	PadPattern string
	// ExpiresAt nil: never (where the policy allows).
	ExpiresAt *time.Time
	Limits    map[string]int64
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrNoName
	}
	if utf8.RuneCountInString(name) > MaxName {
		return "", ErrNameTooLong
	}
	return name, nil
}

// current is the actor as it is now (its account active, its rights read
// again), so a change cannot use rights taken away meanwhile.
func (m *Manager) current(actor accounts.Actor) (accounts.Actor, error) {
	if actor.IsServer() {
		return actor, nil
	}
	return m.accounts.CurrentActor(actor.ID)
}

// holder returns the active account a token is or would be for.
func (m *Manager) holder(accountID string) (*db.Account, error) {
	a, err := m.accounts.Holder(accountID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrNoAccount
	}
	return a, nil
}

// mayManage reports whether actor may manage the tokens of account: its
// own with tokens:own (people only), any with admin:apikeys for accounts
// whose rights the actor all has.
func (m *Manager) mayManage(actor accounts.Actor, a *db.Account) error {
	if actor.IsServer() {
		return nil
	}
	if actor.Grants.HasGlobal(accounts.ScopeAdminAPIKeys) {
		ok, err := m.accounts.MayManage(actor, a.ID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		return ErrForbidden
	}
	if actor.ID == a.ID && a.Kind == accounts.KindHuman && actor.Grants.HasGlobal(accounts.ScopeTokensOwn) {
		return nil
	}
	return ErrForbidden
}

// checkRights checks a token's scopes, pattern and limits against what the
// actor may give.
func checkRights(actor accounts.Actor, scopes []string, pattern string, limits map[string]int64) error {
	if len(scopes) == 0 {
		return ErrNoScopes
	}
	if pattern != "" && accounts.ValidatePattern(pattern) != nil {
		return ErrInvalidPattern
	}
	for _, s := range scopes {
		if s != accounts.AllScopes && !accounts.KnownScope(s) {
			return ErrUnknownScope
		}
		if !actor.IsServer() && !actor.Grants.MayGive(s, pattern) {
			return ErrEscalation
		}
	}
	for _, v := range limits {
		if v < 0 {
			return ErrInvalidLimit
		}
	}
	return nil
}

// checkExpiry applies the policy of the account's kind.
func (m *Manager) checkExpiry(a *db.Account, expires *time.Time) error {
	p := m.policy().Personal
	if a.Kind == accounts.KindService {
		p = m.policy().Service
	}
	now := m.now()
	if expires == nil {
		if p.RequireExpiry || p.MaxLifetimeDays > 0 {
			return ErrExpiryRequired
		}
		return nil
	}
	if !expires.After(now) {
		return ErrExpiryPast
	}
	if p.MaxLifetimeDays > 0 && expires.After(now.Add(time.Duration(p.MaxLifetimeDays)*24*time.Hour)) {
		return ErrExpiryTooLate
	}
	return nil
}

func normalizeScopes(scopes []string) []string {
	out := slices.Clone(scopes)
	slices.Sort(out)
	out = slices.Compact(out)
	if slices.Contains(out, accounts.AllScopes) {
		return []string{accounts.AllScopes}
	}
	return out
}

func newKey() string { return KeyPrefix + utils.RandomString(keyLength) }

func shown(key string) string {
	if len(key) > shownPrefix {
		return key[:shownPrefix]
	}
	return key
}

// Issue makes a token and returns its key, which cannot be recovered later.
func (m *Manager) Issue(actor accounts.Actor, spec Spec) (string, *db.Token, error) {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	actor, err := m.current(actor)
	if err != nil {
		return "", nil, err
	}
	name, err := cleanName(spec.Name)
	if err != nil {
		return "", nil, err
	}
	a, err := m.holder(spec.AccountID)
	if err != nil {
		return "", nil, err
	}
	if err := m.mayManage(actor, a); err != nil {
		return "", nil, err
	}
	scopes := normalizeScopes(spec.Scopes)
	pattern := spec.PadPattern
	if pattern == "*" {
		pattern = ""
	}
	if err := checkRights(actor, scopes, pattern, spec.Limits); err != nil {
		return "", nil, err
	}
	if err := m.checkExpiry(a, spec.ExpiresAt); err != nil {
		return "", nil, err
	}
	key := newKey()
	t := db.Token{ID: utils.RandomString(16), AccountID: a.ID, Name: name, Hash: Hash(key), Prefix: shown(key),
		Scopes: scopes, PadPattern: pattern, ExpiresAt: spec.ExpiresAt, CreatedBy: actor.ID, CreatedAt: m.now(),
		Limits: spec.Limits}
	if err := m.store.CreateToken(t); err != nil {
		return "", nil, err
	}
	m.changed()
	return key, &t, m.accounts.Record(actor, "token.issue", t.ID, map[string]any{
		"account": a.ID, "name": name, "scopes": scopes, "pads": pattern, "expires": expiryText(spec.ExpiresAt)})
}

func expiryText(t *time.Time) string {
	if t == nil {
		return "never"
	}
	return t.UTC().Format(time.RFC3339)
}

// managed loads a token and checks that actor may manage it.
func (m *Manager) managed(actor accounts.Actor, id string) (*db.Token, *db.Account, error) {
	t, err := m.store.GetToken(id)
	if err != nil {
		return nil, nil, err
	}
	if t == nil {
		return nil, nil, ErrTokenNotFound
	}
	a, err := m.accounts.Account(t.AccountID)
	if err != nil && !errors.Is(err, db.ErrAccountNotFound) {
		return nil, nil, err
	}
	if a == nil {
		// The account is gone: only admins of tokens may still clean up.
		if actor.IsServer() || actor.Grants.HasGlobal(accounts.ScopeAdminAPIKeys) {
			return t, &db.Account{ID: t.AccountID, Kind: accounts.KindService}, nil
		}
		return nil, nil, ErrForbidden
	}
	if err := m.mayManage(actor, a); err != nil {
		return nil, nil, err
	}
	return t, a, nil
}

// active reports whether a token can still be used at now.
func active(t *db.Token, now time.Time) bool {
	return t.RevokedAt == nil && (t.ExpiresAt == nil || t.ExpiresAt.After(now))
}

// Rotate makes a new key with the same rights. The old one keeps working
// until oldUntil (nil or past: not at all from now), at most as long as it
// would have. The new key lives as long as the old one was issued for,
// within the policy.
func (m *Manager) Rotate(actor accounts.Actor, id string, oldUntil *time.Time) (string, *db.Token, error) {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	actor, err := m.current(actor)
	if err != nil {
		return "", nil, err
	}
	old, a, err := m.managed(actor, id)
	if err != nil {
		return "", nil, err
	}
	now := m.now()
	if !active(old, now) || old.ReplacedBy != "" {
		return "", nil, ErrNotActive
	}
	if _, err := m.holder(a.ID); err != nil {
		return "", nil, err
	}
	if err := checkRights(actor, old.Scopes, old.PadPattern, old.Limits); err != nil {
		return "", nil, err
	}
	var expires *time.Time
	if old.ExpiresAt != nil {
		e := now.Add(old.ExpiresAt.Sub(old.CreatedAt))
		expires = &e
	}
	expires = m.withinPolicy(a, expires)
	key := newKey()
	t := db.Token{ID: utils.RandomString(16), AccountID: old.AccountID, Name: old.Name, Hash: Hash(key), Prefix: shown(key),
		Scopes: old.Scopes, PadPattern: old.PadPattern, ExpiresAt: expires, Predecessor: old.ID, ClientID: old.ClientID,
		CreatedBy: actor.ID, CreatedAt: now, Limits: old.Limits}
	if err := m.store.CreateToken(t); err != nil {
		return "", nil, err
	}
	until := now
	if oldUntil != nil && oldUntil.After(now) {
		until = *oldUntil
	}
	if old.ExpiresAt != nil && old.ExpiresAt.Before(until) {
		until = *old.ExpiresAt
	}
	old.ExpiresAt, old.ReplacedBy = &until, t.ID
	if err := m.store.UpdateToken(*old); err != nil {
		return "", nil, err
	}
	m.changed()
	return key, &t, m.accounts.Record(actor, "token.rotate", old.ID, map[string]any{
		"new": t.ID, "oldUntil": expiryText(&until), "expires": expiryText(expires)})
}

// withinPolicy brings an expiry within the policy: the longest allowed
// where it is later or "never" is not allowed.
func (m *Manager) withinPolicy(a *db.Account, expires *time.Time) *time.Time {
	p := m.policy().Personal
	if a.Kind == accounts.KindService {
		p = m.policy().Service
	}
	if p.MaxLifetimeDays > 0 {
		limit := m.now().Add(time.Duration(p.MaxLifetimeDays) * 24 * time.Hour)
		if expires == nil || expires.After(limit) {
			return &limit
		}
	}
	if expires == nil && p.RequireExpiry {
		limit := m.now().Add(365 * 24 * time.Hour)
		return &limit
	}
	return expires
}

// SetExpiry changes when an active token stops working (nil: never), e.g.
// to shorten or lengthen the grace period of a rotated key, within the
// policy.
func (m *Manager) SetExpiry(actor accounts.Actor, id string, expires *time.Time) (*db.Token, error) {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	actor, err := m.current(actor)
	if err != nil {
		return nil, err
	}
	t, a, err := m.managed(actor, id)
	if err != nil {
		return nil, err
	}
	if !active(t, m.now()) {
		return nil, ErrNotActive
	}
	if err := m.checkExpiry(a, expires); err != nil {
		return nil, err
	}
	t.ExpiresAt = expires
	if err := m.store.UpdateToken(*t); err != nil {
		return nil, err
	}
	m.changed()
	return t, m.accounts.Record(actor, "token.expiry", t.ID, map[string]any{"expires": expiryText(expires)})
}

// Revoke ends a token at once.
func (m *Manager) Revoke(actor accounts.Actor, id string) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	actor, err := m.current(actor)
	if err != nil {
		return err
	}
	t, _, err := m.managed(actor, id)
	if err != nil {
		return err
	}
	if t.RevokedAt != nil {
		return nil
	}
	now := m.now()
	t.RevokedAt = &now
	if err := m.store.UpdateToken(*t); err != nil {
		return err
	}
	m.changed()
	return m.accounts.Record(actor, "token.revoke", t.ID, map[string]any{"account": t.AccountID, "name": t.Name})
}

// List returns the tokens of an account ("" for all), for an actor who may
// see them: its own with tokens:own, all with admin:apikeys.
func (m *Manager) List(actor accounts.Actor, accountID string) ([]db.Token, error) {
	actor, err := m.current(actor)
	if err != nil {
		return nil, err
	}
	admin := actor.IsServer() || actor.Grants.HasGlobal(accounts.ScopeAdminAPIKeys)
	if !admin {
		if accountID != actor.ID || !actor.Grants.HasGlobal(accounts.ScopeTokensOwn) {
			return nil, ErrForbidden
		}
	}
	list, err := m.store.ListTokens(accountID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Hash = ""
	}
	return list, nil
}

// changed forgets the cached tokens here and, through the accounts'
// notice, on the other instances.
func (m *Manager) changed() {
	m.mu.Lock()
	m.cache = map[string]cachedToken{}
	m.mu.Unlock()
	m.accounts.Changed()
}

// Use is a checked key: the token, its account and what it may do.
type Use struct {
	Token   *db.Token
	Account *db.Account
	Grants  accounts.Grants
}

// Authenticate checks a key and records its use (at most every minute, with
// the IP). It fails with ErrInvalid for an unknown, revoked or expired key
// and for one whose account is gone or disabled. The rights are the
// account's of now, cut down to the token's.
func (m *Manager) Authenticate(key, ip string) (*Use, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrInvalid
	}
	t, err := m.lookup(Hash(key))
	if err != nil {
		return nil, err
	}
	now := m.now()
	if t == nil || !active(t, now) {
		return nil, ErrInvalid
	}
	a, err := m.accounts.Holder(t.AccountID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrInvalid
	}
	g, err := m.accounts.GrantsFor(a.ID)
	if err != nil {
		return nil, err
	}
	m.touch(t, now, ip)
	return &Use{Token: t, Account: a, Grants: g.Restrict(t.Scopes, t.PadPattern, t.Limits)}, nil
}

// lookup finds a token by hash, cached; the cache holds what was read under
// the accounts' current generation only, so every change (also another
// instance's) is seen at once.
func (m *Manager) lookup(hash string) (*db.Token, error) {
	gen := m.accounts.Generation()
	m.mu.Lock()
	if c, ok := m.cache[hash]; ok && c.gen == gen && m.now().Sub(c.at) < cacheTTL {
		m.mu.Unlock()
		return c.token, nil
	}
	m.mu.Unlock()
	t, err := m.store.GetTokenByHash(hash)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	if m.accounts.Generation() == gen {
		if len(m.cache) >= maxCached {
			m.cache = map[string]cachedToken{}
		}
		m.cache[hash] = cachedToken{token: t, at: m.now(), gen: gen}
	}
	m.mu.Unlock()
	return t, nil
}

func (m *Manager) touch(t *db.Token, now time.Time, ip string) {
	m.mu.Lock()
	last, seen := m.touched[t.ID]
	due := !seen || now.Sub(last) >= touchEvery
	if due {
		if len(m.touched) >= maxCached {
			m.touched = map[string]time.Time{}
		}
		m.touched[t.ID] = now
	}
	m.mu.Unlock()
	if due {
		_ = m.store.TouchToken(t.ID, now, ip)
	}
}

// Store returns the data store of the tokens (for tests and tools).
func (m *Manager) Store() db.DataStore {
	ds, _ := m.store.(db.DataStore)
	return ds
}

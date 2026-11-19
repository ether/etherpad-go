package accounts

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
)

// Every change below runs under m.wmu and is checked against the state
// after it: who is still an admin, what the actor may give. The public
// methods lock; the lowercase ones expect the lock held.

func validUsername(u string) bool {
	if u == "" || utf8.RuneCountInString(u) > MaxUsernameLength || strings.TrimSpace(u) != u {
		return false
	}
	// "@" starts the names of the built-in accounts.
	return !strings.HasPrefix(u, "@") && !strings.ContainsAny(u, ":\x00\n\r\t")
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

// RandomPassword returns a password for a one-time login.
func RandomPassword() string { return utils.RandomString(20) }

// state is what the admin rule looks at.
type state struct {
	roles       map[string]db.Role
	accounts    []db.Account
	assignments []db.RoleAssignment
}

func (m *Manager) currentState() (state, error) {
	roles, err := m.Roles()
	if err != nil {
		return state{}, err
	}
	accounts, err := m.store.ListAccounts()
	if err != nil {
		return state{}, err
	}
	assignments, err := m.store.ListRoleAssignments("")
	if err != nil {
		return state{}, err
	}
	return state{roles: roles, accounts: accounts, assignments: assignments}, nil
}

// admins counts the active people who hold the admin role's rights for all
// pads.
func (st state) admins() int {
	active := map[string]bool{}
	for _, a := range st.accounts {
		if a.Kind == KindHuman && a.DisabledAt == nil {
			active[a.ID] = true
		}
	}
	counted := map[string]bool{}
	for _, as := range st.assignments {
		if !active[as.AccountID] || as.PadPattern != "" || counted[as.AccountID] {
			continue
		}
		if _, all := closureScopes(st.roles, as.RoleID); all {
			counted[as.AccountID] = true
		}
	}
	return len(counted)
}

// keepsAnAdmin refuses a change after which no active admin is left, when
// there was one before.
func keepsAnAdmin(before, after state) error {
	if before.admins() > 0 && after.admins() == 0 {
		return ErrLastAdmin
	}
	return nil
}

func (st state) withoutAccount(id string) state {
	out := state{roles: st.roles}
	for _, a := range st.accounts {
		if a.ID != id {
			out.accounts = append(out.accounts, a)
		}
	}
	for _, as := range st.assignments {
		if as.AccountID != id {
			out.assignments = append(out.assignments, as)
		}
	}
	return out
}

func (st state) withDisabled(id string) state {
	out := state{roles: st.roles, assignments: st.assignments}
	for _, a := range st.accounts {
		if a.ID == id {
			now := a.CreatedAt
			a.DisabledAt = &now
		}
		out.accounts = append(out.accounts, a)
	}
	return out
}

func (st state) withoutAssignment(accountID, roleID, pattern string) state {
	out := state{roles: st.roles, accounts: st.accounts}
	for _, as := range st.assignments {
		if !(as.AccountID == accountID && as.RoleID == roleID && as.PadPattern == pattern) {
			out.assignments = append(out.assignments, as)
		}
	}
	return out
}

func (st state) withRoles(roles map[string]db.Role) state {
	return state{roles: roles, accounts: st.accounts, assignments: st.assignments}
}

func (st state) withoutRole(id string) state {
	roles := make(map[string]db.Role, len(st.roles))
	for k, r := range st.roles {
		if k == id {
			continue
		}
		var kept []string
		for _, inc := range r.Includes {
			if inc != id {
				kept = append(kept, inc)
			}
		}
		r.Includes = kept
		roles[k] = r
	}
	out := state{roles: roles, accounts: st.accounts}
	for _, as := range st.assignments {
		if as.RoleID != id {
			out.assignments = append(out.assignments, as)
		}
	}
	return out
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

// limitsAllowed reports whether the actor may give these limits: none
// above its own, unless it has every right.
func limitsAllowed(actor Grants, limits map[string]int64) bool {
	if actor.isAll() {
		return true
	}
	for k, v := range limits {
		own, ok := actor.Limits[k]
		if !ok || v > own {
			return false
		}
	}
	return true
}

// canManage reports whether the actor may act on the account's person:
// change its password, disable or delete it. Only who has all its rights
// may, so nobody takes over an account with more rights than their own.
func (m *Manager) canManage(actor Actor, st state, accountID string) bool {
	if actor.ID == accountID {
		return true
	}
	for _, as := range st.assignments {
		if as.AccountID != accountID {
			continue
		}
		set, all := closureScopes(st.roles, as.RoleID)
		if !allows(actor.Grants, set, all) {
			return false
		}
	}
	return true
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

func uniqueSorted(s []string) []string {
	set := map[string]bool{}
	for _, x := range s {
		if x = strings.TrimSpace(x); x != "" {
			set[x] = true
		}
	}
	return sortedKeys(set)
}

// IsAdmin reports whether the account holds the admin role for all pads.
func (m *Manager) IsAdmin(accountID string) (bool, error) {
	st, err := m.currentState()
	if err != nil {
		return false, err
	}
	for _, as := range st.assignments {
		if as.AccountID == accountID && as.PadPattern == "" {
			if _, all := closureScopes(st.roles, as.RoleID); all {
				return true, nil
			}
		}
	}
	return false, nil
}

// CreateAccount creates a person's or a service account.
func (m *Manager) CreateAccount(actor Actor, n NewAccount) (*db.Account, error) {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	return m.createAccount(actor, n)
}

func (m *Manager) createAccount(actor Actor, n NewAccount) (*db.Account, error) {
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
	return &a, m.audit(actor, "account.create", a.ID, map[string]string{"username": a.Username, "kind": a.Kind})
}

// SetPassword sets a person's password; actor is the person or someone
// with all its rights and admin:users.
func (m *Manager) SetPassword(actor Actor, accountID, password string, mustChange bool) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	if actor.ID != accountID && !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	st, err := m.currentState()
	if err != nil {
		return err
	}
	if !m.canManage(actor, st, accountID) {
		return ErrEscalation
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
	return m.audit(actor, "account.password", a.ID, nil)
}

// ResetPassword gives a person a new one-time password that must be
// changed at the next login, enables the account and lifts a login lock of
// this process. It returns the password.
func (m *Manager) ResetPassword(actor Actor, accountID string) (string, error) {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return "", ErrForbidden
	}
	st, err := m.currentState()
	if err != nil {
		return "", err
	}
	if !m.canManage(actor, st, accountID) {
		return "", ErrEscalation
	}
	a, err := m.store.GetAccount(accountID)
	if err != nil {
		return "", err
	}
	if a == nil {
		return "", db.ErrAccountNotFound
	}
	if a.Kind != KindHuman {
		return "", ErrNoPassword
	}
	password := RandomPassword()
	if a.PasswordHash, err = HashPassword(password); err != nil {
		return "", err
	}
	a.MustChangePassword = true
	a.DisabledAt = nil
	if err := m.store.UpdateAccount(*a); err != nil {
		return "", err
	}
	m.mu.Lock()
	delete(m.throttle, a.Username)
	m.mu.Unlock()
	m.Invalidate()
	return password, m.audit(actor, "account.reset-password", a.ID, nil)
}

// SetDisabled disables or enables an account.
func (m *Manager) SetDisabled(actor Actor, accountID string, disabled bool) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	if isPseudo(accountID) {
		return ErrAnonymousAccount
	}
	st, err := m.currentState()
	if err != nil {
		return err
	}
	if !m.canManage(actor, st, accountID) {
		return ErrEscalation
	}
	a, err := m.store.GetAccount(accountID)
	if err != nil {
		return err
	}
	if a == nil {
		return db.ErrAccountNotFound
	}
	if disabled {
		if err := keepsAnAdmin(st, st.withDisabled(accountID)); err != nil {
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
	return m.audit(actor, map[bool]string{true: "account.disable", false: "account.enable"}[disabled], a.ID, nil)
}

// DeleteAccount removes an account and its assignments.
func (m *Manager) DeleteAccount(actor Actor, accountID string) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	if isPseudo(accountID) {
		return ErrAnonymousAccount
	}
	st, err := m.currentState()
	if err != nil {
		return err
	}
	if !m.canManage(actor, st, accountID) {
		return ErrEscalation
	}
	if err := keepsAnAdmin(st, st.withoutAccount(accountID)); err != nil {
		return err
	}
	if err := m.store.DeleteAccount(accountID); err != nil {
		return err
	}
	m.Invalidate()
	return m.audit(actor, "account.delete", accountID, nil)
}

// SaveRole creates the role (no ID yet, or an unknown one) or changes it.
func (m *Manager) SaveRole(actor Actor, r db.Role) (*db.Role, error) {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	return m.saveRole(actor, r)
}

func (m *Manager) saveRole(actor Actor, r db.Role) (*db.Role, error) {
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
	st, err := m.currentState()
	if err != nil {
		return nil, err
	}
	existing, known := st.roles[r.ID]
	if r.ID == "" {
		r.ID = "role." + utils.RandomString(12)
	}
	r.Builtin = known && existing.Builtin
	r.Scopes = uniqueSorted(r.Scopes)
	r.Includes = uniqueSorted(r.Includes)
	for _, inc := range r.Includes {
		if _, ok := st.roles[inc]; !ok || inc == r.ID {
			return nil, ErrRoleCycle
		}
	}
	next := make(map[string]db.Role, len(st.roles)+1)
	for k, v := range st.roles {
		next[k] = v
	}
	next[r.ID] = r
	if hasCycle(next, r.ID) {
		return nil, ErrRoleCycle
	}
	// What the role gives after the change must be the actor's to give;
	// so must what it gave before (nobody takes rights from a role they
	// could not have handed out), and its limits.
	set, all := closureScopes(next, r.ID)
	if !allows(actor.Grants, set, all) || !limitsAllowed(actor.Grants, r.Limits) {
		return nil, ErrEscalation
	}
	if known {
		oldSet, oldAll := closureScopes(st.roles, r.ID)
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
	if err := keepsAnAdmin(st, st.withRoles(next)); err != nil {
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
	return &r, m.audit(actor, map[bool]string{true: "role.update", false: "role.create"}[known], r.ID,
		map[string]any{"name": r.Name, "scopes": r.Scopes, "includes": r.Includes, "limits": r.Limits})
}

// DeleteRole removes a role that is not built in.
func (m *Manager) DeleteRole(actor Actor, id string) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	if !actor.Grants.HasGlobal(ScopeAdminRoles) {
		return ErrForbidden
	}
	if id == RoleAdmin {
		return ErrAdminRoleFixed
	}
	st, err := m.currentState()
	if err != nil {
		return err
	}
	r, ok := st.roles[id]
	if !ok {
		return db.ErrRoleNotFound
	}
	if r.Builtin {
		return ErrBuiltinRole
	}
	set, all := closureScopes(st.roles, id)
	if !allows(actor.Grants, set, all) {
		return ErrEscalation
	}
	// Also the admins holding a role that includes this one count.
	if err := keepsAnAdmin(st, st.withoutRole(id)); err != nil {
		return err
	}
	if err := m.store.DeleteRole(id); err != nil {
		return err
	}
	m.Invalidate()
	return m.audit(actor, "role.delete", id, map[string]string{"name": r.Name})
}

// Assign gives an account a role for the pads matching padPattern ("" or
// "*": all pads).
func (m *Manager) Assign(actor Actor, accountID, roleID, padPattern string) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	return m.assign(actor, accountID, roleID, padPattern)
}

func (m *Manager) assign(actor Actor, accountID, roleID, padPattern string) error {
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
	return m.audit(actor, "assignment.add", accountID, map[string]string{"role": roleID, "pads": padPattern})
}

// Unassign takes a role assignment away.
func (m *Manager) Unassign(actor Actor, accountID, roleID, padPattern string) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	if padPattern == "*" {
		padPattern = ""
	}
	st, err := m.currentState()
	if err != nil {
		return err
	}
	set, all := closureScopes(st.roles, roleID)
	if !allows(actor.Grants, set, all) {
		return ErrEscalation
	}
	if err := keepsAnAdmin(st, st.withoutAssignment(accountID, roleID, padPattern)); err != nil {
		return err
	}
	if err := m.store.RemoveRoleAssignment(accountID, roleID, padPattern); err != nil {
		return err
	}
	m.Invalidate()
	return m.audit(actor, "assignment.remove", accountID, map[string]string{"role": roleID, "pads": padPattern})
}

var errUnknownTemplate = errors.New("unknown template")

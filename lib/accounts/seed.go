package accounts

import (
	"errors"
	"fmt"
	"sort"

	"github.com/ether/etherpad-go/lib/db"
	"go.uber.org/zap"
)

// BuiltinRoles are the roles every instance starts with. Admins change
// them (except admin) and add their own.
func BuiltinRoles() []db.Role {
	return []db.Role{
		{ID: RoleViewer, Name: "viewer", Description: "Read pads", Builtin: true, Scopes: []string{ScopePadRead}},
		{ID: RoleEditor, Name: "editor", Description: "Edit and create pads, write in the chat", Builtin: true,
			Scopes: []string{ScopePadWrite, ScopePadCreate, ScopeChatWrite}, Includes: []string{RoleViewer}},
		{ID: RoleModerator, Name: "moderator", Description: "Delete pads, enforce pad settings, moderate the chat", Builtin: true,
			Scopes: []string{ScopePadDelete, ScopePadSettings, ScopeChatModerate}, Includes: []string{RoleEditor}},
		{ID: RoleAIUser, Name: "ai-user", Description: "Use the AI assistant, nothing else", Builtin: true,
			Scopes: []string{ScopeAIUse}},
		{ID: RoleAdmin, Name: "admin", Description: "Everything, including scopes added later", Builtin: true,
			Scopes:   []string{ScopeAdminUsers, ScopeAdminRoles, ScopeAdminAPIKeys, ScopeAdminSettings, ScopeAdminPlugins, ScopeAdminPads, ScopeTokensOwn, ScopeAIUnlimited},
			Includes: []string{RoleModerator, RoleAIUser}},
	}
}

// Template is what visitors who are not logged in may do on a new
// instance.
type Template string

const (
	// TemplateOpen lets visitors read, write and create pads.
	TemplateOpen Template = "open"
	// TemplateRead lets visitors read pads only.
	TemplateRead Template = "read"
	// TemplateClosed lets visitors do nothing: everyone logs in.
	TemplateClosed Template = "closed"
)

// LegacyUser is an entry of settings.users, taken over once.
type LegacyUser struct {
	Password  string
	IsAdmin   bool
	ReadOnly  bool
	CanCreate *bool
}

// SeedConfig is what EnsureDefaults sets up an instance from.
type SeedConfig struct {
	// Template applies when the anonymous account is created, i.e. on
	// the first start with account management.
	Template Template
	// LegacyUsers are taken over when there are no accounts of people
	// yet.
	LegacyUsers map[string]LegacyUser
	// BootstrapUser and BootstrapPassword create the first admin when no
	// active admin exists.
	BootstrapUser     string
	BootstrapPassword string
}

// TemplateOf returns the template that keeps etherpad-lite's behaviour of
// requireAuthentication.
func TemplateOf(requireAuthentication bool) Template {
	if requireAuthentication {
		return TemplateClosed
	}
	return TemplateOpen
}

func ignoreConflict(err error) error {
	if errors.Is(err, db.ErrConflict) {
		return nil
	}
	return err
}

// legacyMarker is the audit entry that records that settings.users was
// looked at; it is written once, so settings.users is taken over once.
const legacyMarker = "migration.settings-users"

// EnsureDefaults prepares the account management of an instance; it is
// safe to run on every start and from several instances at once:
//   - the built-in roles that are missing are created;
//   - the anonymous and authenticated accounts are created, and on
//     creation get the roles of the template;
//   - settings.users is taken over exactly once (the first start records
//     that it did, even without users); an interrupted take-over is
//     completed on the next start;
//   - without an admin who can log in, the bootstrap admin is created.
func (m *Manager) EnsureDefaults(cfg SeedConfig, logger *zap.SugaredLogger) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	for _, r := range BuiltinRoles() {
		if err := ignoreConflict(m.store.CreateRole(r)); err != nil {
			return err
		}
	}
	m.Invalidate()

	createdAnon, err := m.ensurePseudo(AnonymousID, "Visitors (not logged in)")
	if err != nil {
		return err
	}
	createdAuth, err := m.ensurePseudo(AuthenticatedID, "Everyone logged in")
	if err != nil {
		return err
	}

	done, err := m.store.HasAuditEntry(legacyMarker)
	if err != nil {
		return err
	}
	if !done {
		if err := m.importLegacy(cfg.LegacyUsers, logger); err != nil {
			return err
		}
		err := m.store.AppendAudit(db.AuditEntry{ID: legacyMarker, At: m.now(), ActorID: System.ID,
			Action: "migration.settings-users", Target: "settings.users", Details: fmt.Sprintf(`{"users":%d}`, len(cfg.LegacyUsers))})
		if err := ignoreConflict(err); err != nil {
			return err
		}
	} else if len(cfg.LegacyUsers) > 0 && logger != nil {
		logger.Warn("settings.users is no longer read: its users were taken over into the account management; remove the block from settings.json and manage accounts in the admin UI or with \"etherpad-go admin\"")
	}

	if createdAnon {
		switch cfg.Template {
		case TemplateOpen:
			err = m.assign(System, AnonymousID, RoleEditor, "")
		case TemplateRead:
			err = m.assign(System, AnonymousID, RoleViewer, "")
		}
		if err := ignoreConflict(err); err != nil {
			return err
		}
	}
	if createdAuth && cfg.Template == TemplateClosed {
		if !restrictedLegacy(cfg.LegacyUsers) {
			// Behind a login, everyone logged in could edit and create pads.
			if err := ignoreConflict(m.assign(System, AuthenticatedID, RoleEditor, "")); err != nil {
				return err
			}
		} else if logger != nil {
			logger.Warn("settings.users had read-only users or users who may not create pads: people logged in only get the roles of their own accounts")
		}
	}

	return m.ensureAdmin(cfg, logger)
}

// restrictedLegacy reports whether settings.users limited some user: then
// logged-in people as a whole cannot get editor, or those users would get
// it too.
func restrictedLegacy(users map[string]LegacyUser) bool {
	for _, u := range users {
		if !u.IsAdmin && (u.ReadOnly || (u.CanCreate != nil && !*u.CanCreate)) {
			return true
		}
	}
	return false
}

func (m *Manager) ensurePseudo(id, name string) (bool, error) {
	err := m.store.CreateAccount(db.Account{ID: id, Username: "@" + id, DisplayName: name, Kind: KindBuiltin, CreatedAt: m.now()})
	if errors.Is(err, db.ErrConflict) {
		return false, nil
	}
	return err == nil, err
}

// noCreateRole is the role of settings.users with canCreate: false.
const noCreateRole = "editor-no-create"

func legacyRole(u LegacyUser) string {
	switch {
	case u.IsAdmin:
		return RoleAdmin
	case u.ReadOnly:
		return RoleViewer
	case u.CanCreate != nil && !*u.CanCreate:
		return noCreateRole
	}
	return RoleEditor
}

// importLegacy takes settings.users over. A user whose account exists
// without any role (a take-over interrupted between the two writes, or by
// another instance) gets its role; an account with roles is left alone.
func (m *Manager) importLegacy(users map[string]LegacyUser, logger *zap.SugaredLogger) error {
	names := make([]string, 0, len(users))
	for name := range users {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		u := users[name]
		if !validUsername(name) {
			if logger != nil {
				logger.Warnf("settings.users: %q is not a valid username and was not taken over", name)
			}
			continue
		}
		role := legacyRole(u)
		if role == noCreateRole {
			if err := ignoreConflict(m.store.CreateRole(db.Role{
				ID: noCreateRole, Name: noCreateRole, Description: "Edit pads but not create them (from settings.users)",
				Scopes: []string{ScopePadWrite, ScopeChatWrite}, Includes: []string{RoleViewer},
			})); err != nil {
				return err
			}
			m.Invalidate()
		}
		a, err := m.store.GetAccountByUsername(name)
		if err != nil {
			return err
		}
		if a == nil {
			hash := ""
			if u.Password != "" {
				if hash, err = HashPassword(u.Password); err != nil {
					return err
				}
			}
			a, err = m.createAccount(System, NewAccount{Username: name, Kind: KindHuman, PasswordHash: hash})
			if errors.Is(err, db.ErrConflict) {
				if a, err = m.store.GetAccountByUsername(name); err != nil || a == nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
		refs, err := m.Refs(a.ID)
		if err != nil {
			return err
		}
		if len(refs) > 0 {
			continue
		}
		if err := ignoreConflict(m.assign(System, a.ID, role, "")); err != nil {
			return err
		}
		if logger != nil {
			logger.Infof("settings.users: took over %q as an account with the role %s", name, role)
		}
	}
	return nil
}

// adminCanLogIn reports whether an active person with a password is an
// admin: an admin without password (like the "admin" of
// settings.template.json) lets nobody into the admin UI.
func (st state) adminCanLogIn() bool {
	withPassword := state{roles: st.roles, assignments: st.assignments}
	for _, a := range st.accounts {
		if a.PasswordHash != "" {
			withPassword.accounts = append(withPassword.accounts, a)
		}
	}
	return withPassword.admins() > 0
}

func (m *Manager) ensureAdmin(cfg SeedConfig, logger *zap.SugaredLogger) error {
	st, err := m.currentState()
	if err != nil {
		return err
	}
	if st.adminCanLogIn() {
		return nil
	}
	if cfg.BootstrapUser == "" || cfg.BootstrapPassword == "" {
		if logger != nil {
			logger.Warn("There is no admin who can log in. Create one with \"etherpad-go admin create <username>\" (it prints a one-time password), or set EP_BOOTSTRAP_ADMIN_USER and EP_BOOTSTRAP_ADMIN_PASSWORD")
		}
		return nil
	}
	if !validUsername(cfg.BootstrapUser) || len([]rune(cfg.BootstrapPassword)) < MinPasswordLength {
		if logger != nil {
			logger.Errorf("The bootstrap admin was not created: the username is invalid or the password has fewer than %d characters", MinPasswordLength)
		}
		return nil
	}
	a, err := m.store.GetAccountByUsername(cfg.BootstrapUser)
	if err != nil {
		return err
	}
	switch {
	case a == nil:
		a, err = m.createAccount(System, NewAccount{
			Username: cfg.BootstrapUser, Kind: KindHuman, Password: cfg.BootstrapPassword, MustChangePassword: true,
		})
		if errors.Is(err, db.ErrConflict) {
			return nil // another instance created it, and makes it an admin
		}
		if err != nil {
			return err
		}
	case a.Kind != KindHuman || a.DisabledAt != nil:
		if logger != nil {
			logger.Errorf("The bootstrap admin %q exists as a disabled or service account and was not changed", cfg.BootstrapUser)
		}
		return nil
	case a.PasswordHash == "":
		// An account nobody can log in to (like the "admin" without
		// password of settings.template.json): it gets the bootstrap
		// password, to be changed at the first login.
		if a.PasswordHash, err = HashPassword(cfg.BootstrapPassword); err != nil {
			return err
		}
		a.MustChangePassword = true
		if err := m.store.UpdateAccount(*a); err != nil {
			return err
		}
		m.Invalidate()
	case a.MustChangePassword && CheckPassword(a.PasswordHash, cfg.BootstrapPassword):
		// The bootstrap admin of an earlier start that stopped before it
		// got its role: finish it.
	default:
		if logger != nil {
			logger.Warnf("The bootstrap admin %q already exists with its own password and was not changed; use \"etherpad-go admin reset-password %s\" to get in", cfg.BootstrapUser, cfg.BootstrapUser)
		}
		return nil
	}
	if err := ignoreConflict(m.assign(System, a.ID, RoleAdmin, "")); err != nil {
		return err
	}
	if logger != nil {
		logger.Infof("The bootstrap admin %q can log in; its password must be changed at the first login", cfg.BootstrapUser)
	}
	return nil
}

// ApplyTemplate replaces the roles of the anonymous and authenticated
// accounts with those of a template: open lets visitors edit, read lets
// them read, closed lets only logged-in people in (and lets them edit).
func (m *Manager) ApplyTemplate(actor Actor, t Template) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
	}
	var target, role string
	switch t {
	case TemplateOpen:
		target, role = AnonymousID, RoleEditor
	case TemplateRead:
		target, role = AnonymousID, RoleViewer
	case TemplateClosed:
		target, role = AuthenticatedID, RoleEditor
	default:
		return fmt.Errorf("%w %q", errUnknownTemplate, t)
	}
	roles, err := m.Roles()
	if err != nil {
		return err
	}
	set, all := closureScopes(roles, role)
	if !allows(actor.Grants, set, all) {
		return ErrEscalation
	}
	for _, id := range []string{AnonymousID, AuthenticatedID} {
		as, err := m.store.ListRoleAssignments(id)
		if err != nil {
			return err
		}
		for _, a := range as {
			if err := m.store.RemoveRoleAssignment(id, a.RoleID, a.PadPattern); err != nil {
				return err
			}
		}
	}
	m.Invalidate()
	if err := m.assign(actor, target, role, ""); err != nil {
		return err
	}
	return m.audit(actor, "template.apply", AnonymousID, map[string]string{"template": string(t)})
}

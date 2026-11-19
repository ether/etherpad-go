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

// EnsureDefaults prepares the account management of an instance; it is
// safe to run on every start and from several instances at once:
//   - the built-in roles that are missing are created;
//   - the anonymous and authenticated accounts are created, and on
//     creation get the roles of the template;
//   - settings.users are taken over once, while no person has an account;
//   - without an active admin, the bootstrap admin is created.
func (m *Manager) EnsureDefaults(cfg SeedConfig, logger *zap.SugaredLogger) error {
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

	accounts, err := m.store.ListAccounts()
	if err != nil {
		return err
	}
	people := 0
	for _, a := range accounts {
		if a.Kind == KindHuman {
			people++
		}
	}
	if people == 0 && len(cfg.LegacyUsers) > 0 {
		if err := m.importLegacy(cfg.LegacyUsers, logger); err != nil {
			return err
		}
	} else if people > 0 && len(cfg.LegacyUsers) > 0 && logger != nil {
		logger.Warn("settings.users is no longer read: its users were taken over into the account management; remove the block from settings.json and manage accounts in the admin UI")
	}

	if createdAnon {
		switch cfg.Template {
		case TemplateOpen:
			err = m.Assign(System, AnonymousID, RoleEditor, "")
		case TemplateRead:
			err = m.Assign(System, AnonymousID, RoleViewer, "")
		}
		if err != nil {
			return err
		}
	}
	if createdAuth && cfg.Template == TemplateClosed && !restrictedLegacy(cfg.LegacyUsers) {
		// Behind a login, everyone logged in could edit and create pads.
		if err := m.Assign(System, AuthenticatedID, RoleEditor, ""); err != nil {
			return err
		}
	} else if createdAuth && cfg.Template == TemplateClosed && logger != nil {
		logger.Warn("settings.users had read-only users or users who may not create pads: people logged in only get the roles of their own accounts")
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
		hash := ""
		if u.Password != "" {
			var err error
			if hash, err = HashPassword(u.Password); err != nil {
				return err
			}
		}
		a, err := m.CreateAccount(System, NewAccount{Username: name, Kind: KindHuman, PasswordHash: hash})
		if errors.Is(err, db.ErrConflict) {
			continue // another instance took it over
		}
		if err != nil {
			return err
		}
		role := RoleEditor
		switch {
		case u.IsAdmin:
			role = RoleAdmin
		case u.ReadOnly:
			role = RoleViewer
		case u.CanCreate != nil && !*u.CanCreate:
			if err := ignoreConflict(m.store.CreateRole(db.Role{
				ID: noCreateRole, Name: noCreateRole, Description: "Edit pads but not create them (from settings.users)",
				Scopes: []string{ScopePadWrite, ScopeChatWrite}, Includes: []string{RoleViewer},
			})); err != nil {
				return err
			}
			m.Invalidate()
			role = noCreateRole
		}
		if err := m.Assign(System, a.ID, role, ""); err != nil && !errors.Is(err, db.ErrConflict) {
			return err
		}
		if logger != nil {
			logger.Infof("settings.users: took over %q as an account with the role %s", name, role)
		}
	}
	return nil
}

func (m *Manager) ensureAdmin(cfg SeedConfig, logger *zap.SugaredLogger) error {
	roles, err := m.Roles()
	if err != nil {
		return err
	}
	// An admin who cannot log in with a password (like the "admin"
	// without password of settings.template.json) does not count: then
	// nobody could get into the admin UI.
	if ok, err := m.adminWithPassword(roles); err != nil || ok {
		return err
	}
	if cfg.BootstrapUser == "" || cfg.BootstrapPassword == "" {
		if logger != nil {
			logger.Warn("There is no admin who can log in. Create one with \"etherpad-go admin create <username>\" (it prints a one-time password), or set EP_BOOTSTRAP_ADMIN_USER and EP_BOOTSTRAP_ADMIN_PASSWORD")
		}
		return nil
	}
	if existing, err := m.store.GetAccountByUsername(cfg.BootstrapUser); err != nil || existing != nil {
		if err == nil && logger != nil {
			logger.Warnf("The bootstrap admin %q already exists as an account without admin rights; it was not changed", cfg.BootstrapUser)
		}
		return err
	}
	a, err := m.CreateAccount(System, NewAccount{
		Username: cfg.BootstrapUser, Kind: KindHuman, Password: cfg.BootstrapPassword, MustChangePassword: true,
	})
	if errors.Is(err, db.ErrConflict) {
		return nil // another instance created it
	}
	if errors.Is(err, ErrPasswordTooShort) || errors.Is(err, ErrInvalidUsername) {
		if logger != nil {
			logger.Errorf("The bootstrap admin was not created: %v (passwords need at least %d characters)", err, MinPasswordLength)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if err := m.Assign(System, a.ID, RoleAdmin, ""); err != nil {
		return err
	}
	if logger != nil {
		logger.Infof("Created the bootstrap admin %q; its password must be changed at the first login", cfg.BootstrapUser)
	}
	return nil
}

// ApplyTemplate replaces the roles of the anonymous and authenticated
// accounts with those of a template: open lets visitors edit, read lets
// them read, closed lets only logged-in people in (and lets them edit).
func (m *Manager) ApplyTemplate(actor Actor, t Template) error {
	if !actor.Grants.HasGlobal(ScopeAdminUsers) {
		return ErrForbidden
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
	var err error
	switch t {
	case TemplateOpen:
		err = m.Assign(actor, AnonymousID, RoleEditor, "")
	case TemplateRead:
		err = m.Assign(actor, AnonymousID, RoleViewer, "")
	case TemplateClosed:
		err = m.Assign(actor, AuthenticatedID, RoleEditor, "")
	default:
		err = fmt.Errorf("unknown template %q", t)
	}
	if err != nil {
		return err
	}
	m.audit(actor, "template.apply", AnonymousID, map[string]string{"template": string(t)})
	return nil
}

// adminWithPassword reports whether an active person with a password is an
// admin.
func (m *Manager) adminWithPassword(roles map[string]db.Role) (bool, error) {
	accounts, err := m.store.ListAccounts()
	if err != nil {
		return false, err
	}
	for _, a := range accounts {
		if a.Kind != KindHuman || a.DisabledAt != nil || a.PasswordHash == "" {
			continue
		}
		if ok, err := m.isAdminAccount(roles, a.ID, nil); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

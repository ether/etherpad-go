package accounts

import (
	"reflect"
	"sync"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"go.uber.org/zap"
)

var registry sync.Map // store pointer → *Manager

// For returns the manager of store: one per store and process, so every
// part of the server sees the same cache.
func For(store db.AccountMethods) *Manager {
	v := reflect.ValueOf(store)
	if v.Kind() != reflect.Pointer {
		return NewManager(store)
	}
	if m, ok := registry.Load(v.Pointer()); ok {
		return m.(*Manager)
	}
	m, _ := registry.LoadOrStore(v.Pointer(), NewManager(store))
	return m.(*Manager)
}

// SeedConfigOf builds the seed configuration of an instance from its
// settings: the template from requireAuthentication, settings.users, and
// the bootstrap admin.
func SeedConfigOf(cfg *settings.Settings) SeedConfig {
	sc := SeedConfig{Template: TemplateOpen}
	if cfg == nil {
		return sc
	}
	sc.Template = TemplateOf(cfg.RequireAuthentication)
	sc.BootstrapUser, sc.BootstrapPassword = cfg.BootstrapAdminCredentials()
	if len(cfg.Users) > 0 {
		sc.LegacyUsers = make(map[string]LegacyUser, len(cfg.Users))
		for name, u := range cfg.Users {
			lu := LegacyUser{CanCreate: u.CanCreate}
			if u.Password != nil {
				lu.Password = *u.Password
			}
			lu.IsAdmin = u.IsAdmin != nil && *u.IsAdmin
			lu.ReadOnly = u.ReadOnly != nil && *u.ReadOnly
			sc.LegacyUsers[name] = lu
		}
	}
	return sc
}

// Ensure runs EnsureDefaults once per manager with the settings of the
// first caller; later calls only wait for it. The server calls it at
// startup; code paths that can run first (tests, tools) call it too.
func (m *Manager) Ensure(cfg *settings.Settings, logger *zap.SugaredLogger) error {
	m.seedMu.Lock()
	defer m.seedMu.Unlock()
	if m.seeded {
		return nil
	}
	if err := m.EnsureDefaults(SeedConfigOf(cfg), logger); err != nil {
		return err
	}
	m.seeded = true
	return nil
}

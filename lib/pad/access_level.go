package pad

import (
	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/settings"
	"net/url"
)

// AccountsOf returns the account manager of store, set up from cfg on
// first use.
func AccountsOf(store db.DataStore, cfg *settings.Settings) *accounts.Manager {
	m := accounts.For(store)
	_ = m.Ensure(cfg, nil)
	return m
}

// requestGrants returns what the request's user may do; nil is a visitor
// who is not logged in. A user without resolved grants (made by code that
// predates accounts) gets those of its account by name, else those of
// everyone logged in. Errors grant nothing.
func requestGrants(m *accounts.Manager, user *webaccess.SocketClientRequest) accounts.Grants {
	if user == nil {
		g, _ := m.AnonymousGrants()
		return g
	}
	if user.Grants != nil {
		return *user.Grants
	}
	if user.Username != nil {
		if a, err := m.AccountByUsername(*user.Username); err == nil && a != nil && a.DisabledAt == nil && a.Kind == accounts.KindHuman {
			g, _ := m.GrantsFor(a.ID)
			return g
		}
	}
	g, _ := m.AuthenticatedGrants()
	return g
}

// sessionUserOf is the request user of a logged-in account.
func sessionUserOf(m *accounts.Manager, username string, a *db.Account) *webaccess.SocketClientRequest {
	name := username
	user := &webaccess.SocketClientRequest{Username: &name}
	var g accounts.Grants
	if a != nil && a.DisabledAt == nil && a.Kind == accounts.KindHuman {
		user.AccountID = a.ID
		g, _ = m.GrantsFor(a.ID)
	} else {
		g, _ = m.AuthenticatedGrants()
	}
	user.Grants = &g
	user.IsAdmin = g.IsAdmin()
	return user
}

var levelRank = map[string]int{accounts.LevelNone: 0, accounts.LevelReadOnly: 1, accounts.LevelModify: 2, accounts.LevelCreate: 3}

func minLevel(a, b string) string {
	if levelRank[a] <= levelRank[b] {
		return a
	}
	return b
}

// capByFlags applies the readOnly and canCreate restrictions a request
// user may carry.
func capByFlags(level string, user *webaccess.SocketClientRequest) string {
	if user == nil {
		return level
	}
	if user.ReadOnly != nil && *user.ReadOnly {
		level = minLevel(level, accounts.LevelReadOnly)
	}
	if user.CanCreate != nil && !*user.CanCreate {
		level = minLevel(level, accounts.LevelModify)
	}
	return level
}

// padLevel decides the authorization level of user (nil: a visitor) on the
// real pad padId: the level its grants give; admins get create; with
// requireAuthorization, a logged-in person's level is also limited by what
// the authorize hooks grant, and no answer denies, as in etherpad-lite.
func padLevel(m *accounts.Manager, user *webaccess.SocketClientRequest, padId, path string, cfg *settings.Settings, hookSystem *hooks.Hook) string {
	g := requestGrants(m, user)
	if user != nil && (user.IsAdmin || g.IsAdmin()) {
		return accounts.LevelCreate
	}
	level := capByFlags(g.Level(padId), user)
	if level == accounts.LevelNone || user == nil || cfg == nil || !cfg.RequireAuthorization {
		return level
	}
	if hookSystem == nil {
		return accounts.LevelNone
	}
	if path == "" {
		path = "/p/" + url.PathEscape(padId)
	}
	authorizeCtx := &events.AuthorizeContext{Path: path, PadId: padId, User: user}
	hookSystem.ExecuteAuthorizeHooks(authorizeCtx)
	if authorizeCtx.Decision() != events.AuthorizeGrant {
		return accounts.LevelNone
	}
	hookLevel, err := NormalizeAuthzLevel(authorizeCtx.Level())
	if err != nil {
		return accounts.LevelNone
	}
	return minLevel(level, *hookLevel)
}

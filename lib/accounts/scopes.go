// Package accounts is etherpad-go's account management: accounts (people,
// service accounts and the anonymous visitor), roles built from scopes,
// role assignments limited to pad patterns, and the rules that keep an
// instance from locking itself out or opening up by mistake.
//
// Code checks scopes, never role names: "may this account do X on this
// pad?" is Grants.Has(X, padId).
package accounts

import (
	"slices"
	"sort"
	"strings"
	"sync"
)

// The scopes of etherpad-go. Plugins add their own with RegisterScope.
const (
	ScopePadRead       = "pad:read"
	ScopePadWrite      = "pad:write"
	ScopePadCreate     = "pad:create"
	ScopePadDelete     = "pad:delete"
	ScopePadSettings   = "pad:settings"
	ScopeChatWrite     = "chat:write"
	ScopeChatModerate  = "chat:moderate"
	ScopeAIUse         = "ai:use"
	ScopeAIUnlimited   = "ai:unlimited"
	ScopeTokensOwn     = "tokens:own"
	ScopeAdminUsers    = "admin:users"
	ScopeAdminRoles    = "admin:roles"
	ScopeAdminAPIKeys  = "admin:apikeys"
	ScopeAdminSettings = "admin:settings"
	ScopeAdminPlugins  = "admin:plugins"
	ScopeAdminPads     = "admin:pads"
)

// Scope describes one right.
type Scope struct {
	Name        string
	Description string
	// PadScoped scopes can be limited to a pad pattern; the others only
	// count from assignments for all pads.
	PadScoped bool
}

var (
	scopesMu sync.RWMutex
	scopes   = map[string]Scope{}
)

func init() {
	for _, s := range []Scope{
		{ScopePadRead, "Read pads and sheets, export them, see their history", true},
		{ScopePadWrite, "Edit pads and sheets, import, restore versions", true},
		{ScopePadCreate, "Create pads", true},
		{ScopePadDelete, "Delete pads, clean up their history", true},
		{ScopePadSettings, "Enforce pad settings", true},
		{ScopeChatWrite, "Write in the pad chat", true},
		{ScopeChatModerate, "Delete chat messages", true},
		{ScopeAIUse, "Use the AI assistant in pads", true},
		{ScopeAIUnlimited, "Use the AI without a daily limit", false},
		{ScopeTokensOwn, "Create own tokens", false},
		{ScopeAdminUsers, "Manage accounts and their roles", false},
		{ScopeAdminRoles, "Manage roles", false},
		{ScopeAdminAPIKeys, "Issue and revoke tokens of any account", false},
		{ScopeAdminSettings, "Change the settings of the instance", false},
		{ScopeAdminPlugins, "Install and remove plugins", false},
		{ScopeAdminPads, "List and manage all pads of the instance", false},
	} {
		scopes[s.Name] = s
	}
}

// RegisterScope adds a scope, e.g. one of a plugin. Registering a known
// name again replaces its description.
func RegisterScope(s Scope) {
	scopesMu.Lock()
	defer scopesMu.Unlock()
	scopes[s.Name] = s
}

// LookupScope returns the scope with this name.
func LookupScope(name string) (Scope, bool) {
	scopesMu.RLock()
	defer scopesMu.RUnlock()
	s, ok := scopes[name]
	return s, ok
}

// Scopes returns all known scopes, by name.
func Scopes() []Scope {
	scopesMu.RLock()
	defer scopesMu.RUnlock()
	out := make([]Scope, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func scopeNames() []string {
	all := Scopes()
	out := make([]string, len(all))
	for i, s := range all {
		out[i] = s.Name
	}
	return out
}

// padScoped reports whether name can be limited to a pad pattern. Unknown
// scopes (of a plugin not loaded right now) count as pad scoped, so they
// are never widened to all pads.
func padScoped(name string) bool {
	s, ok := LookupScope(name)
	return !ok || s.PadScoped
}

// IsAdminScope reports whether name is one of the admin:* scopes.
func IsAdminScope(name string) bool { return strings.HasPrefix(name, "admin:") }

// anonymousForbidden are the scopes the anonymous account can never have.
func anonymousForbidden(name string) bool {
	return IsAdminScope(name) || slices.Contains([]string{ScopeTokensOwn, ScopeAIUnlimited}, name)
}

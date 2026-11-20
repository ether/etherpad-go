package accounts

import (
	"slices"
	"sort"

	"github.com/ether/etherpad-go/lib/db"
)

// The ids of the built-in roles and of the anonymous account.
const (
	RoleViewer    = "viewer"
	RoleEditor    = "editor"
	RoleModerator = "moderator"
	RoleAIUser    = "ai-user"
	RoleAdmin     = "admin"

	// AnonymousID is the account of visitors who are not logged in; every
	// account gets its roles too.
	AnonymousID = "anonymous"
	// AuthenticatedID is the account of everyone logged in, also people a
	// plugin authenticated who have no account of their own.
	AuthenticatedID = "authenticated"
)

func isPseudo(id string) bool { return id == AnonymousID || id == AuthenticatedID }

// Account kinds.
const (
	KindHuman   = "human"
	KindService = "service"
	// KindBuiltin marks the anonymous and authenticated accounts.
	KindBuiltin = "builtin"
)

// The levels of pad access of etherpad-lite's authorization.
const (
	LevelNone     = ""
	LevelReadOnly = "readOnly"
	LevelModify   = "modify"
	LevelCreate   = "create"
)

type grant struct {
	pattern string
	scopes  map[string]bool
	// all: the admin role, every scope including those registered later.
	all bool
}

// Grants are the resolved rights of an account: scopes per pad pattern.
// The zero value grants nothing.
type Grants struct {
	grants []grant
	// Limits are the most generous value of each limit over the roles.
	Limits map[string]int64
	// inner and restrict: these grants are inner's, cut down to restrict
	// (a token's scopes and pad pattern).
	inner    *Grants
	restrict *restriction
	// alts are grants that count as well (a union with cut-down grants).
	alts []Grants
}

// restriction is what a token may use of its account's rights.
type restriction struct {
	all     bool
	scopes  map[string]bool
	pattern string
}

func (r *restriction) scope(s string) bool { return r.all || r.scopes[s] }

// pad reports whether the restriction lets scope reach padId.
func (r *restriction) pad(scope, padId string) bool {
	return r.pattern == "" || (padScoped(scope) && MatchPattern(r.pattern, padId))
}

// AllScopes in a token's scope list means every scope, also those
// registered later.
const AllScopes = "*"

// Restrict cuts the grants down to scopes, a pad pattern ("" or "*": all
// pads) and limits: what a token of the account may do. A token never gets
// more than its account: every check needs both. A limit is the smaller of
// the account's and the token's; scopes that are not pad scoped need a
// token for all pads, as they need an assignment for all pads.
func (g Grants) Restrict(scopes []string, pattern string, limits map[string]int64) Grants {
	r := &restriction{scopes: map[string]bool{}, pattern: pattern}
	if pattern == "*" {
		r.pattern = ""
	}
	for _, s := range scopes {
		if s == AllScopes {
			r.all = true
		}
		r.scopes[s] = true
	}
	inner := g
	out := Grants{inner: &inner, restrict: r, Limits: map[string]int64{}}
	for k, v := range g.Limits {
		out.Limits[k] = v
	}
	for k, v := range limits {
		// Only lower what the account has: a limit the account lacks is
		// not given by its token.
		if cur, ok := out.Limits[k]; ok && v < cur {
			out.Limits[k] = v
		}
	}
	return out
}

func (g grant) has(scope string) bool { return g.all || g.scopes[scope] }

// Has reports whether the grants allow scope on padId. Scopes that are not
// pad scoped only count from assignments for all pads.
func (g Grants) Has(scope, padId string) bool {
	for _, a := range g.alts {
		if a.Has(scope, padId) {
			return true
		}
	}
	if g.inner != nil {
		return g.restrict.scope(scope) && g.restrict.pad(scope, padId) && g.inner.Has(scope, padId)
	}
	scoped := padScoped(scope)
	for _, gr := range g.grants {
		if !gr.has(scope) {
			continue
		}
		if gr.pattern == "" {
			return true
		}
		if scoped && MatchPattern(gr.pattern, padId) {
			return true
		}
	}
	return false
}

// HasGlobal reports whether the grants allow scope on every pad (or, for
// scopes that are not pad scoped, at all).
func (g Grants) HasGlobal(scope string) bool {
	for _, a := range g.alts {
		if a.HasGlobal(scope) {
			return true
		}
	}
	if g.inner != nil {
		return g.restrict.scope(scope) && g.restrict.pattern == "" && g.inner.HasGlobal(scope)
	}
	for _, gr := range g.grants {
		if gr.pattern == "" && gr.has(scope) {
			return true
		}
	}
	return false
}

// HasAnywhere reports whether the grants allow scope on at least some pads.
func (g Grants) HasAnywhere(scope string) bool {
	for _, a := range g.alts {
		if a.HasAnywhere(scope) {
			return true
		}
	}
	scoped := padScoped(scope)
	if g.inner != nil {
		return g.restrict.scope(scope) && (g.restrict.pattern == "" || scoped) && g.inner.HasAnywhere(scope)
	}
	for _, gr := range g.grants {
		if gr.has(scope) && (gr.pattern == "" || scoped) {
			return true
		}
	}
	return false
}

// IsAdmin reports whether the grants include the admin role's rights: the
// admin UI needs admin:settings.
func (g Grants) IsAdmin() bool { return g.HasGlobal(ScopeAdminSettings) }

// Empty reports whether the grants allow nothing at all.
func (g Grants) Empty() bool {
	for _, a := range g.alts {
		if !a.Empty() {
			return false
		}
	}
	if g.inner != nil {
		return (!g.restrict.all && len(g.restrict.scopes) == 0) || g.inner.Empty()
	}
	for _, gr := range g.grants {
		if gr.all || len(gr.scopes) > 0 {
			return false
		}
	}
	return true
}

// Level is the etherpad-lite authorization level the grants give on padId:
// create needs pad:create and pad:write, modify pad:write, readOnly
// pad:read.
func (g Grants) Level(padId string) string {
	switch {
	case !g.Has(ScopePadRead, padId) && !g.Has(ScopePadWrite, padId):
		return LevelNone
	case !g.Has(ScopePadWrite, padId):
		return LevelReadOnly
	case !g.Has(ScopePadCreate, padId):
		return LevelModify
	}
	return LevelCreate
}

// Union returns the grants of g and o together.
func (g Grants) Union(o Grants) Grants {
	out := Grants{grants: append(slices.Clone(g.grants), o.grants...), Limits: map[string]int64{}}
	if g.inner != nil || o.inner != nil || len(g.alts) > 0 || len(o.alts) > 0 {
		// Cut-down grants keep their cut: both count side by side.
		out = Grants{alts: []Grants{g, o}, Limits: map[string]int64{}}
	}
	for _, src := range []map[string]int64{g.Limits, o.Limits} {
		for k, v := range src {
			if cur, ok := out.Limits[k]; !ok || v > cur {
				out.Limits[k] = v
			}
		}
	}
	return out
}

// GlobalScopes returns the scopes granted for all pads, by name.
func (g Grants) GlobalScopes() []string {
	set := map[string]bool{}
	for _, a := range g.alts {
		for _, s := range a.GlobalScopes() {
			set[s] = true
		}
	}
	if g.inner != nil {
		if g.restrict.pattern == "" {
			for _, s := range g.inner.GlobalScopes() {
				if g.restrict.scope(s) {
					set[s] = true
				}
			}
		}
		return sortedKeys(set)
	}
	for _, gr := range g.grants {
		if gr.pattern != "" {
			continue
		}
		if gr.all {
			for _, s := range scopeNames() {
				set[s] = true
			}
		}
		for s := range gr.scopes {
			set[s] = true
		}
	}
	return sortedKeys(set)
}

// PatternScopes is the scope list of one pad pattern ("" = all pads).
type PatternScopes struct {
	Pattern string   `json:"pattern"`
	Scopes  []string `json:"scopes"`
}

// Summary lists the scopes per pattern, for showing what an account may do.
func (g Grants) Summary() []PatternScopes {
	byPattern := map[string]map[string]bool{}
	add := func(pattern, scope string) {
		set := byPattern[pattern]
		if set == nil {
			set = map[string]bool{}
			byPattern[pattern] = set
		}
		set[scope] = true
	}
	for _, a := range g.alts {
		for _, ps := range a.Summary() {
			for _, s := range ps.Scopes {
				add(ps.Pattern, s)
			}
		}
	}
	if g.inner != nil {
		for _, ps := range g.inner.Summary() {
			pattern := ps.Pattern
			if r := g.restrict.pattern; r != "" {
				// Both patterns must match; shown side by side.
				if pattern == "" {
					pattern = r
				} else if pattern != r {
					pattern += " & " + r
				}
			}
			for _, s := range ps.Scopes {
				if g.restrict.scope(s) && (g.restrict.pattern == "" || padScoped(s)) {
					add(pattern, s)
				}
			}
		}
		return summaryOf(byPattern)
	}
	for _, gr := range g.grants {
		set := byPattern[gr.pattern]
		if set == nil {
			set = map[string]bool{}
			byPattern[gr.pattern] = set
		}
		if gr.all {
			for _, s := range scopeNames() {
				set[s] = true
			}
		}
		for s := range gr.scopes {
			if gr.pattern == "" || padScoped(s) {
				set[s] = true
			}
		}
	}
	return summaryOf(byPattern)
}

func summaryOf(byPattern map[string]map[string]bool) []PatternScopes {
	out := make([]PatternScopes, 0, len(byPattern))
	for p, set := range byPattern {
		if len(set) > 0 {
			out = append(out, PatternScopes{Pattern: p, Scopes: sortedKeys(set)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pattern < out[j].Pattern })
	return out
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// roleClosure returns the role and every role it includes, directly or not.
// Cycles (which the manager refuses to store) end the walk.
func roleClosure(roles map[string]db.Role, id string) []db.Role {
	var out []db.Role
	seen := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		r, ok := roles[id]
		if !ok {
			return
		}
		out = append(out, r)
		for _, inc := range r.Includes {
			walk(inc)
		}
	}
	walk(id)
	return out
}

// RoleRef is an assignment as a token carries it: a role id and the pad
// pattern ("" = all pads).
type RoleRef struct {
	Role string `json:"role"`
	Pads string `json:"pads,omitempty"`
}

// resolve turns assignments into grants with the role definitions.
func resolve(roles map[string]db.Role, refs []RoleRef) Grants {
	g := Grants{Limits: map[string]int64{}}
	for _, ref := range refs {
		gr := grant{pattern: ref.Pads, scopes: map[string]bool{}}
		if ref.Pads == "*" {
			gr.pattern = ""
		}
		for _, r := range roleClosure(roles, ref.Role) {
			if r.ID == RoleAdmin {
				gr.all = true
			}
			for _, s := range r.Scopes {
				gr.scopes[s] = true
			}
			for k, v := range r.Limits {
				if cur, ok := g.Limits[k]; !ok || v > cur {
					g.Limits[k] = v
				}
			}
		}
		g.grants = append(g.grants, gr)
	}
	return g
}

// closureScopes returns every scope of the role and the roles it includes;
// all reports the admin role among them.
func closureScopes(roles map[string]db.Role, id string) (set map[string]bool, all bool) {
	set = map[string]bool{}
	for _, r := range roleClosure(roles, id) {
		if r.ID == RoleAdmin {
			all = true
		}
		for _, s := range r.Scopes {
			set[s] = true
		}
	}
	return set, all
}

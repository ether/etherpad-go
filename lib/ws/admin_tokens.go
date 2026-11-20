package ws

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/tokens"
)

// TokenView is a token as the admin page lists it: never its key or hash.
type TokenView struct {
	ID          string           `json:"id"`
	AccountID   string           `json:"accountId"`
	Name        string           `json:"name"`
	Prefix      string           `json:"prefix"`
	Scopes      []string         `json:"scopes"`
	Pads        string           `json:"pads"`
	ExpiresAt   *int64           `json:"expiresAt"`
	RevokedAt   *int64           `json:"revokedAt"`
	ReplacedBy  string           `json:"replacedBy,omitempty"`
	Predecessor string           `json:"predecessor,omitempty"`
	CreatedBy   string           `json:"createdBy"`
	CreatedAt   int64            `json:"createdAt"`
	LastUsedAt  *int64           `json:"lastUsedAt"`
	LastUsedIP  string           `json:"lastUsedIp,omitempty"`
	Limits      map[string]int64 `json:"limits"`
	// Status: active, grace (replaced, still working until it expires),
	// expired or revoked.
	Status string `json:"status"`
}

// TokenAccount is an account tokens can be issued for, with the scopes it
// has somewhere (what the form offers and preselects).
type TokenAccount struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Kind     string   `json:"kind"`
	Disabled bool     `json:"disabled"`
	Scopes   []string `json:"scopes"`
}

type TokenPolicyView struct {
	RequireExpiry   bool `json:"requireExpiry"`
	MaxLifetimeDays int  `json:"maxLifetimeDays"`
}

// TokensResult answers every token event with the whole list; for a new
// key (issue, rotate) the key itself, which is not shown again.
type TokensResult struct {
	Tokens   []TokenView     `json:"tokens"`
	Accounts []TokenAccount  `json:"accounts"`
	Scopes   []ScopeView     `json:"scopes"`
	Personal TokenPolicyView `json:"personal"`
	Service  TokenPolicyView `json:"service"`
	// Names maps account ids (owners, issuers) to usernames.
	Names   map[string]string `json:"names"`
	Created *TokenCreated     `json:"created,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type TokenCreated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

func isTokenEvent(event string) bool {
	switch event {
	case "tokensLoad", "tokenIssue", "tokenRotate", "tokenSetExpiry", "tokenRevoke":
		return true
	}
	return false
}

var tokenErrors = []error{
	tokens.ErrNoName, tokens.ErrNameTooLong, tokens.ErrUnknownScope, tokens.ErrNoScopes, tokens.ErrForbidden,
	tokens.ErrEscalation, tokens.ErrNoAccount, tokens.ErrExpiryRequired, tokens.ErrExpiryTooLate, tokens.ErrExpiryPast,
	tokens.ErrNotActive, tokens.ErrInvalidLimit, tokens.ErrInvalidPattern, tokens.ErrTokenNotFound,
	accounts.ErrForbidden, accounts.ErrAuditFailed,
}

func msPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

func timeOfMs(ms *int64) *time.Time {
	if ms == nil {
		return nil
	}
	t := time.UnixMilli(*ms)
	return &t
}

func tokenStatus(t db.Token, now time.Time) string {
	switch {
	case t.RevokedAt != nil:
		return "revoked"
	case t.ExpiresAt != nil && !t.ExpiresAt.After(now):
		return "expired"
	case t.ReplacedBy != "":
		return "grace"
	}
	return "active"
}

// handleTokenEvent manages the tokens. Every event is answered with
// "results:tokens".
func (h AdminMessageHandler) handleTokenEvent(event string, data json.RawMessage, cfg *settings.Settings, c *Client) {
	acc := accounts.For(h.store)
	keys := tokens.For(h.store, cfg)
	result := TokensResult{Tokens: []TokenView{}, Accounts: []TokenAccount{}, Scopes: []ScopeView{}, Names: map[string]string{}}
	actor, err := h.actor(acc, c)
	if err == nil && !actor.Grants.HasGlobal(accounts.ScopeAdminAPIKeys) {
		err = accounts.ErrForbidden
	}
	if err != nil {
		result.Error = h.tokenError(event, err)
		h.sendResult(c, "results:tokens", result)
		return
	}
	result.Created, err = h.applyTokenEvent(keys, actor, event, data)
	if err != nil {
		result.Error = h.tokenError(event, err)
	}
	if err := h.fillTokens(acc, keys, actor, cfg, &result); err != nil {
		h.Logger.Errorf("admin: listing tokens: %v", err)
		if result.Error == "" {
			result.Error = "the tokens could not be loaded"
		}
	}
	h.sendResult(c, "results:tokens", result)
}

func (h AdminMessageHandler) tokenError(event string, err error) string {
	for _, known := range tokenErrors {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntax) || errors.As(err, &typeErr) {
		return "invalid request"
	}
	h.Logger.Errorf("admin: %s: %v", event, err)
	return "the change could not be made"
}

func (h AdminMessageHandler) applyTokenEvent(keys *tokens.Manager, actor accounts.Actor, event string, data json.RawMessage) (*TokenCreated, error) {
	var req struct {
		ID        string           `json:"id"`
		AccountID string           `json:"accountId"`
		Name      string           `json:"name"`
		Scopes    []string         `json:"scopes"`
		Pads      string           `json:"pads"`
		ExpiresAt *int64           `json:"expiresAt"`
		OldUntil  *int64           `json:"oldUntil"`
		Limits    map[string]int64 `json:"limits"`
	}
	if event != "tokensLoad" && len(data) > 0 {
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
	}
	switch event {
	case "tokenIssue":
		key, t, err := keys.Issue(actor, tokens.Spec{AccountID: req.AccountID, Name: req.Name, Scopes: req.Scopes,
			PadPattern: req.Pads, ExpiresAt: timeOfMs(req.ExpiresAt), Limits: req.Limits})
		if t == nil {
			return nil, err
		}
		h.Logger.Infof("admin: token %s (%s) issued for %s by %s", t.Prefix, t.Name, t.AccountID, actor.ID)
		return &TokenCreated{ID: t.ID, Name: t.Name, Key: key}, err
	case "tokenRotate":
		key, t, err := keys.Rotate(actor, req.ID, timeOfMs(req.OldUntil))
		if t == nil {
			return nil, err
		}
		h.Logger.Infof("admin: token %s rotated to %s by %s", req.ID, t.Prefix, actor.ID)
		return &TokenCreated{ID: t.ID, Name: t.Name, Key: key}, err
	case "tokenSetExpiry":
		_, err := keys.SetExpiry(actor, req.ID, timeOfMs(req.ExpiresAt))
		return nil, err
	case "tokenRevoke":
		err := keys.Revoke(actor, req.ID)
		if err == nil {
			h.Logger.Infof("admin: token %s revoked by %s", req.ID, actor.ID)
		}
		return nil, err
	}
	return nil, nil
}

func (h AdminMessageHandler) fillTokens(acc *accounts.Manager, keys *tokens.Manager, actor accounts.Actor, cfg *settings.Settings, result *TokensResult) error {
	list, err := keys.List(actor, "")
	if err != nil {
		return err
	}
	people, err := acc.Accounts()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, a := range people {
		result.Names[a.ID] = a.Username
		if a.Kind != accounts.KindHuman && a.Kind != accounts.KindService {
			continue
		}
		v := TokenAccount{ID: a.ID, Username: a.Username, Kind: a.Kind, Disabled: a.DisabledAt != nil, Scopes: []string{}}
		if g, err := acc.GrantsFor(a.ID); err == nil {
			for _, s := range accounts.Scopes() {
				if g.HasAnywhere(s.Name) {
					v.Scopes = append(v.Scopes, s.Name)
				}
			}
		}
		result.Accounts = append(result.Accounts, v)
	}
	sort.Slice(result.Accounts, func(i, j int) bool { return result.Accounts[i].Username < result.Accounts[j].Username })
	for _, t := range list {
		v := TokenView{ID: t.ID, AccountID: t.AccountID, Name: t.Name, Prefix: t.Prefix, Scopes: nonNil(t.Scopes),
			Pads: t.PadPattern, ExpiresAt: msPtr(t.ExpiresAt), RevokedAt: msPtr(t.RevokedAt), ReplacedBy: t.ReplacedBy,
			Predecessor: t.Predecessor, CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt.UnixMilli(),
			LastUsedAt: msPtr(t.LastUsedAt), LastUsedIP: t.LastUsedIP, Limits: t.Limits, Status: tokenStatus(t, now)}
		if v.Limits == nil {
			v.Limits = map[string]int64{}
		}
		result.Tokens = append(result.Tokens, v)
	}
	// Newest first: what was just issued is on top.
	sort.SliceStable(result.Tokens, func(i, j int) bool { return result.Tokens[i].CreatedAt > result.Tokens[j].CreatedAt })
	for _, s := range accounts.Scopes() {
		result.Scopes = append(result.Scopes, ScopeView{Name: s.Name, Description: s.Description, PadScoped: s.PadScoped})
	}
	result.Personal = TokenPolicyView(cfg.Tokens.Personal)
	result.Service = TokenPolicyView(cfg.Tokens.Service)
	return nil
}

package padlogin

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/tokens"
	"github.com/gofiber/fiber/v3"
)

// TokensPath is where people logged in on the pads manage their own
// tokens ("My tokens" in the profile menu); it needs tokens:own.
const TokensPath = "/auth/tokens"

// OwnToken is a token as its owner sees it: never its key or hash.
type OwnToken struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes"`
	Pads       string   `json:"pads"`
	ExpiresAt  *int64   `json:"expiresAt"`
	CreatedAt  int64    `json:"createdAt"`
	LastUsedAt *int64   `json:"lastUsedAt"`
	LastUsedIP string   `json:"lastUsedIp,omitempty"`
	// Status: active, grace (replaced, still working until it expires).
	Status string `json:"status"`
}

// OwnScope is a scope the person has, which a token of theirs may get.
type OwnScope struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PadScoped   bool   `json:"padScoped"`
}

// OwnTokens answers every request of "My tokens": the live tokens, what
// a new one may get, the expiry rules; Key is a new key, shown once.
type OwnTokens struct {
	Tokens          []OwnToken `json:"tokens"`
	Scopes          []OwnScope `json:"scopes"`
	RequireExpiry   bool       `json:"requireExpiry"`
	MaxLifetimeDays int        `json:"maxLifetimeDays"`
	Key             string     `json:"key,omitempty"`
	KeyName         string     `json:"keyName,omitempty"`
}

// tokenErrors are told to the person as they are.
var tokenErrors = []error{
	tokens.ErrNoName, tokens.ErrNameTooLong, tokens.ErrUnknownScope, tokens.ErrNoScopes, tokens.ErrForbidden,
	tokens.ErrEscalation, tokens.ErrExpiryRequired, tokens.ErrExpiryTooLate, tokens.ErrExpiryPast,
	tokens.ErrNotActive, tokens.ErrInvalidLimit, tokens.ErrInvalidPattern, tokens.ErrTokenNotFound,
	tokens.ErrLimitTooHigh, tokens.ErrGraceTooLong,
}

func (l *Login) registerTokens(app fiber.Router) {
	app.Get(TokensPath, l.ownTokens)
	app.Post(TokensPath, l.issueOwnToken)
	app.Post(TokensPath+"/:id/rotate", l.rotateOwnToken)
	app.Post(TokensPath+"/:id/revoke", l.revokeOwnToken)
}

// owner is the logged-in person of the request as a token actor, or an
// answer already sent: 401 without a login, 403 without tokens:own, 403
// for a state-changing request from another site or without JSON (a form
// of another site cannot send JSON without asking first).
func (l *Login) owner(c fiber.Ctx, changes bool) (accounts.Actor, bool, error) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if changes && (!sameOrigin(c) || !strings.HasPrefix(c.Get(fiber.HeaderContentType), fiber.MIMEApplicationJSON)) {
		return accounts.Actor{}, false, c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}
	var a *db.Account
	if c.Cookies(CookieName(l.settings)) != "" {
		a = l.personOf(c)
	}
	if a == nil {
		return accounts.Actor{}, false, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not logged in"})
	}
	actor, err := l.accounts.CurrentActor(a.ID)
	if err != nil {
		return accounts.Actor{}, false, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not logged in"})
	}
	if !actor.Grants.HasGlobal(accounts.ScopeTokensOwn) {
		return accounts.Actor{}, false, c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": tokens.ErrForbidden.Error()})
	}
	return actor, true, nil
}

// fail answers a refused change: the known reasons as they are, others
// logged.
func (l *Login) fail(c fiber.Ctx, err error) error {
	for _, known := range tokenErrors {
		if errors.Is(err, known) {
			status := fiber.StatusBadRequest
			if errors.Is(err, tokens.ErrForbidden) || errors.Is(err, tokens.ErrEscalation) || errors.Is(err, tokens.ErrLimitTooHigh) {
				status = fiber.StatusForbidden
			}
			if errors.Is(err, tokens.ErrTokenNotFound) {
				status = fiber.StatusNotFound
			}
			return c.Status(status).JSON(fiber.Map{"error": known.Error()})
		}
	}
	l.logger.Errorf("my tokens: %v", err)
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "the change could not be made"})
}

func msOf(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

// state is what "My tokens" shows for actor.
func (l *Login) state(actor accounts.Actor) (OwnTokens, error) {
	out := OwnTokens{Tokens: []OwnToken{}, Scopes: []OwnScope{}}
	list, err := l.tokens.List(actor, actor.ID)
	if err != nil {
		return out, err
	}
	now := time.Now()
	for _, t := range list {
		if t.RevokedAt != nil || (t.ExpiresAt != nil && !t.ExpiresAt.After(now)) {
			continue
		}
		status := "active"
		if t.ReplacedBy != "" {
			status = "grace"
		}
		scopes := append([]string{}, t.Scopes...)
		out.Tokens = append(out.Tokens, OwnToken{ID: t.ID, Name: t.Name, Prefix: t.Prefix, Scopes: scopes, Pads: t.PadPattern,
			ExpiresAt: msOf(t.ExpiresAt), CreatedAt: t.CreatedAt.UnixMilli(), LastUsedAt: msOf(t.LastUsedAt),
			LastUsedIP: t.LastUsedIP, Status: status})
	}
	sort.SliceStable(out.Tokens, func(i, j int) bool { return out.Tokens[i].CreatedAt > out.Tokens[j].CreatedAt })
	for _, s := range accounts.Scopes() {
		// What a token may get: scopes the person has for all pads; a
		// pad-scoped one held for some pads only needs a pattern within
		// them, which the server checks on issue.
		if actor.Grants.HasAnywhere(s.Name) {
			out.Scopes = append(out.Scopes, OwnScope{Name: s.Name, Description: s.Description, PadScoped: s.PadScoped})
		}
	}
	p := l.settings.Tokens.Personal
	out.RequireExpiry, out.MaxLifetimeDays = p.RequireExpiry, p.MaxLifetimeDays
	return out, nil
}

func (l *Login) reply(c fiber.Ctx, actor accounts.Actor, key, keyName string) error {
	st, err := l.state(actor)
	if err != nil {
		return l.fail(c, err)
	}
	st.Key, st.KeyName = key, keyName
	return c.JSON(st)
}

func (l *Login) ownTokens(c fiber.Ctx) error {
	actor, ok, err := l.owner(c, false)
	if !ok {
		return err
	}
	return l.reply(c, actor, "", "")
}

func (l *Login) issueOwnToken(c fiber.Ctx) error {
	actor, ok, err := l.owner(c, true)
	if !ok {
		return err
	}
	var req struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		Pads      string   `json:"pads"`
		ExpiresAt *int64   `json:"expiresAt"`
		// Days counts from the server's clock: the browser's may be ahead,
		// which would put the policy's longest lifetime past its limit.
		Days *int `json:"days"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	var expires *time.Time
	if req.Days != nil && *req.Days > 0 {
		t := time.Now().Add(time.Duration(*req.Days) * 24 * time.Hour)
		expires = &t
	} else if req.ExpiresAt != nil {
		t := time.UnixMilli(*req.ExpiresAt)
		expires = &t
	}
	key, t, err := l.tokens.Issue(actor, tokens.Spec{AccountID: actor.ID, Name: req.Name, Scopes: req.Scopes,
		PadPattern: req.Pads, ExpiresAt: expires})
	if t == nil {
		return l.fail(c, err)
	}
	l.logger.Infof("my tokens: %s issued token %s (%s)", actor.ID, t.Prefix, t.Name)
	// Issued even when only the audit failed: the key must be shown.
	return l.reply(c, actor, key, t.Name)
}

func (l *Login) rotateOwnToken(c fiber.Ctx) error {
	actor, ok, err := l.owner(c, true)
	if !ok {
		return err
	}
	var req struct {
		OldUntil *int64 `json:"oldUntil"`
	}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
		}
	}
	var until *time.Time
	if req.OldUntil != nil {
		t := time.UnixMilli(*req.OldUntil)
		until = &t
	}
	if err := l.mine(actor, c.Params("id")); err != nil {
		return l.fail(c, err)
	}
	key, t, err := l.tokens.Rotate(actor, c.Params("id"), until)
	if t == nil {
		return l.fail(c, err)
	}
	return l.reply(c, actor, key, t.Name)
}

func (l *Login) revokeOwnToken(c fiber.Ctx) error {
	actor, ok, err := l.owner(c, true)
	if !ok {
		return err
	}
	if err := l.mine(actor, c.Params("id")); err != nil {
		return l.fail(c, err)
	}
	if err := l.tokens.Revoke(actor, c.Params("id")); err != nil {
		return l.fail(c, err)
	}
	return l.reply(c, actor, "", "")
}

// mine checks that the token is the actor's own: here nobody manages
// another's tokens, admin or not (that is the admin page's).
func (l *Login) mine(actor accounts.Actor, id string) error {
	list, err := l.tokens.List(actor, actor.ID)
	if err != nil {
		return err
	}
	for _, t := range list {
		if t.ID == id {
			return nil
		}
	}
	return tokens.ErrTokenNotFound
}

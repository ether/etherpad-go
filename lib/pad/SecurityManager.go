package pad

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
)

type SecurityManager struct {
	ReadOnlyManager *ReadOnlyManager
	PadManager      *Manager
	AuthorManager   *author.Manager
	SessionManager  *SessionManager
	hooks           *hooks.Hook
	settings        *settings.Settings
}

// NewSecurityManager creates a SecurityManager that reads its access rules
// from cfg instead of the global settings, so tests can run in parallel with
// different settings.
func NewSecurityManager(db db.DataStore, hooks *hooks.Hook, padManager *Manager, cfg *settings.Settings) *SecurityManager {
	return &SecurityManager{
		settings:        cfg,
		ReadOnlyManager: NewReadOnlyManager(db),
		PadManager:      padManager,
		AuthorManager:   author.NewManager(db),
		SessionManager:  NewSessionManager(db),
		hooks:           hooks,
	}
}

type GrantedAccess struct {
	AccessStatus string
	AuthorId     string
}

func (s *SecurityManager) CheckAccess(padId *string, sessionCookie *string, token *string, userSettings *webaccess.SocketClientRequest) (*GrantedAccess, error) {
	if padId == nil {
		return nil, errors.New("padId is nil")
	}
	var canCreate = !s.settings.EditOnly
	if s.ReadOnlyManager.IsReadOnlyID(padId) {
		canCreate = false
		foundPadId, err := s.ReadOnlyManager.GetPadId(*padId)

		if err != nil {
			return nil, errors.New("padId not found")
		}
		padId = foundPadId
	}

	if s.hooks != nil {
		var tok string
		if token != nil {
			tok = *token
		}
		var cookie string
		if sessionCookie != nil {
			cookie = *sessionCookie
		}
		accessCtx := &events.OnAccessCheckContext{PadId: *padId, Token: tok, SessionCookie: cookie}
		s.hooks.ExecuteOnAccessCheckHooks(accessCtx)
		if accessCtx.Denied() {
			return nil, errors.New("access denied: onAccessCheck hook denied access")
		}
	}

	if s.settings.LoadTest {
		authorId, err := s.resolveAuthorId(token, userSettings)
		if err != nil {
			return nil, errors.New("access denied: invalid author token" + err.Error())
		}
		return &GrantedAccess{AccessStatus: "grant", AuthorId: authorId}, nil
	} else if s.settings.RequireAuthentication {
		if userSettings == nil {
			return nil, errors.New("userSettings is nil")
		}
		if !userSettings.CanCreate {
			canCreate = false
		}

		if userSettings.ReadOnly != nil && *userSettings.ReadOnly == true {
			canCreate = false
		}

		var padAuthzs *map[string]string
		if userSettings.PadAuthorizations == nil {
			var padAuthzMap = make(map[string]string)
			padAuthzs = &padAuthzMap
		} else {
			padAuthzs = userSettings.PadAuthorizations
		}
		var unwrappedMap = *padAuthzs
		var entry = unwrappedMap[*padId]
		var level, err = NormalizeAuthzLevel(entry)

		if err != nil {
			return nil, fmt.Errorf("access denied: unauthorized: %w", err)
		}

		if level != nil {
			if *level != "create" {
				canCreate = false
			}
		}
	}

	var padExists, err = s.PadManager.DoesPadExist(*padId)
	if err != nil {
		return nil, fmt.Errorf("internal error while checking pad existence: %w", err)
	}

	if !*padExists && !canCreate {
		return nil, errors.New("pad does not exist and can't be created due to settings")
	}

	var splittedPadId = strings.Split(*padId, "$")[0]

	var sessionAuthorID = s.SessionManager.findAuthorID(splittedPadId, sessionCookie)

	if s.settings.RequireSession && sessionAuthorID == nil {
		return nil, errors.New("access denied: HTTP API session is required")
	}

	// A missing token is fine (resolveAuthorId hands out a fresh one); a
	// malformed one is rejected.
	if sessionAuthorID == nil && token != nil && *token != "" && !utils.IsValidAuthorToken(*token) {
		return nil, errors.New("invalid author token")
	}

	// Resolve the author only once access is granted, so a denied request
	// does not leave an author record behind. An integrator session's author
	// takes precedence over the token's author, as in etherpad-lite.
	grant := func() (*GrantedAccess, error) {
		if sessionAuthorID != nil {
			return &GrantedAccess{AccessStatus: "grant", AuthorId: *sessionAuthorID}, nil
		}
		authorId, err := s.resolveAuthorId(token, userSettings)
		if err != nil {
			return nil, fmt.Errorf("access denied: invalid author token: %w", err)
		}
		return &GrantedAccess{AccessStatus: "grant", AuthorId: authorId}, nil
	}

	if !strings.Contains(*padId, "$") {
		// Only group pads can be private, so there is nothing more to check.
		return grant()
	}

	if !*padExists {
		if sessionAuthorID == nil {
			return nil, errors.New("access denied: must have an HTTP API session to create a group pad")
		}
		// Creating a group pad, so there is no public status to check.
		return grant()
	}

	var pad, _ = s.PadManager.GetPad(*padId, nil, nil)

	if !pad.PublicStatus && sessionAuthorID == nil {
		return nil, errors.New("must have an HTTP API session to access private group pads")
	}

	return grant()
}

// resolveAuthorId resolves the author id for a token, first giving getAuthorId
// hooks a chance to supply/override it (first non-empty wins), then falling back
// to the database token->author mapping.
//
// A missing token reaches the hooks as "" (etherpad-lite: null) but is never
// looked up itself: every token-less caller would share the author mapped to
// the empty token and could impersonate the others, e.g. delete pads they
// created (GHSA-746x-9w89-7xv6). It gets a fresh token instead.
func (s *SecurityManager) resolveAuthorId(token *string, userSettings *webaccess.SocketClientRequest) (string, error) {
	var tok string
	if token != nil {
		tok = *token
	}
	if s.hooks != nil {
		idCtx := &events.GetAuthorIdContext{Token: tok, User: userSettings}
		s.hooks.ExecuteGetAuthorIdHooks(idCtx)
		if idCtx.AuthorId() != "" {
			return idCtx.AuthorId(), nil
		}
	}
	if tok == "" {
		tok = utils.GenerateAuthorToken()
	}
	retrievedAuthor, err := s.AuthorManager.GetAuthorId(tok)
	if err != nil {
		return "", err
	}
	return retrievedAuthor.Id, nil
}

func (s *SecurityManager) HasPadAccess(ctx fiber.Ctx) bool {
	tokenCookie := ctx.Cookies("token")
	padId := ctx.Params("pad")
	accessStatus, err := s.CheckAccess(&padId, nil, &tokenCookie, nil)
	if err != nil {
		return false
	}
	return accessStatus != nil && accessStatus.AccessStatus == "grant"
}

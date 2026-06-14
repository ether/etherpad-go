package pad

import (
	"errors"
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
}

func NewSecurityManager(db db.DataStore, hooks *hooks.Hook, padManager *Manager) *SecurityManager {
	return &SecurityManager{
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
	var canCreate = !settings.Displayed.EditOnly
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

	if settings.Displayed.LoadTest {
		authorId, err := s.resolveAuthorId(token, userSettings)
		if err != nil {
			return nil, errors.New("access denied: invalid author token" + err.Error())
		}
		return &GrantedAccess{AccessStatus: "grant", AuthorId: authorId}, nil
	} else if settings.Displayed.RequireAuthentication {
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
			println("Access denied: unauthorized")
			return nil, err
		}

		if level != nil {
			if *level != "create" {
				canCreate = false
			}
		}
	}

	var padExists, err = s.PadManager.DoesPadExist(*padId)
	if err != nil {
		println("An error occurred while checking pad existence:", err.Error())
		return nil, errors.New("internal error while checking pad existence")
	}

	if !*padExists && !canCreate {
		return nil, errors.New("pad does not exist and can't be created due to settings")
	}

	var splittedPadId = strings.Split(*padId, "$")[0]

	var sessionAuthorID = s.SessionManager.findAuthorID(splittedPadId, sessionCookie)

	if settings.Displayed.RequireSession && sessionAuthorID == nil {
		return nil, errors.New("access denied: HTTP API session is required")
	}

	if sessionAuthorID == nil && token != nil && !utils.IsValidAuthorToken(*token) {
		return nil, errors.New("invalid author token")
	}

	authorId, err := s.resolveAuthorId(token, userSettings)
	if err != nil {
		println("An error occurred while retrieving author from token:", err.Error())
		return nil, errors.New("access denied: invalid author token")
	}
	var grantedAccess = GrantedAccess{AccessStatus: "grant", AuthorId: authorId}

	if !strings.Contains(*padId, "$") {
		return &grantedAccess, nil
	}

	if !*padExists {
		if sessionAuthorID == nil {
			return nil, errors.New("access denied: must have an HTTP API session to create a group pad")
		}
		// Creating a group pad, so there is no public status to check.
		return &grantedAccess, nil
	}

	var pad, _ = s.PadManager.GetPad(*padId, nil, nil)

	if !pad.PublicStatus && sessionAuthorID == nil {
		return nil, errors.New("must have an HTTP API session to access private group pads")
	}

	return &grantedAccess, nil
}

// resolveAuthorId resolves the author id for a token, first giving getAuthorId
// hooks a chance to supply/override it (first non-empty wins), then falling back
// to the database token->author mapping.
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

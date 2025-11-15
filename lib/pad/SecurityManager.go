package pad

import (
	"errors"
	"strings"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
)

type SecurityManager struct {
	ReadOnlyManager *ReadOnlyManager
	PadManager      *Manager
	AuthorManager   *author.Manager
	SessionManager  *SessionManager
}

func NewSecurityManager(db db.DataStore, hooks *hooks.Hook, padManager *Manager) SecurityManager {
	return SecurityManager{
		ReadOnlyManager: NewReadOnlyManager(db),
		PadManager:      padManager,
		AuthorManager:   author.NewManager(db),
		SessionManager:  NewSessionManager(db),
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
	if s.ReadOnlyManager.isReadOnlyID(padId) {
		canCreate = false
		foundPadId := s.ReadOnlyManager.getPadId(*padId)

		if foundPadId == nil {
			return nil, errors.New("padId not found")
		}
		padId = foundPadId
	}

	if settings.Displayed.LoadTest {
		return nil, nil
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

	var padExists = s.PadManager.DoesPadExist(*padId)

	if !padExists && !canCreate {
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

	var grantedAccess = GrantedAccess{
		AccessStatus: "grant",
		AuthorId:     s.AuthorManager.GetAuthorId(*token).Id,
	}

	if !strings.Contains(*padId, "$") {
		return &grantedAccess, nil
	}

	if !padExists {
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

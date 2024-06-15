package pad

import (
	"errors"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"strings"
)

type SecurityManager struct {
	ReadOnlyManager *ReadOnlyManager
	PadManager      Manager
	AuthorManager   author.Manager
}

func NewSecurityManager() SecurityManager {
	return SecurityManager{
		ReadOnlyManager: NewReadOnlyManager(),
		PadManager:      NewManager(),
		AuthorManager:   author.NewManager(),
	}
}

type GrantedAccess struct {
	AccessStatus string
	AuthorId     string
}

func (s *SecurityManager) CheckAccess(padId *string, sessionCookie *string, token *string, userSettings *UserSettings) (*GrantedAccess, error) {
	if padId == nil {
		return nil, errors.New("padId is nil")
	}
	var canCreate = !settings.SettingsDisplayed.EditOnly
	if s.ReadOnlyManager.isReadOnlyID(padId) {
		canCreate = false
		foundPadId := s.ReadOnlyManager.getPadId(*padId)

		if foundPadId == nil {
			return nil, errors.New("padId not found")
		}
		padId = foundPadId
	}

	if settings.SettingsDisplayed.LoadTest {
		return nil, nil
	} else if settings.SettingsDisplayed.RequireAuthentication {
		if userSettings == nil {
			return nil, errors.New("userSettings is nil")
		}
		// TODO implement later
	}

	var padExists = s.PadManager.DoesPadExist(*padId)

	if !padExists && !canCreate {
		return nil, errors.New("pad does not exist and can't be created due to settings")
	}

	if token != nil && !utils.IsValidAuthorToken(*token) {
		return nil, errors.New("Invalid author token")
	}

	var grantedAccess = GrantedAccess{
		AccessStatus: "grant",
		AuthorId:     s.AuthorManager.GetAuthorId(*token),
	}

	if !strings.Contains(*padId, "$") {
		return &grantedAccess, nil
	}

	return &grantedAccess, nil
}

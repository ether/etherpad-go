package pad

import (
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
	"regexp"
	"strings"
)

type SessionManager struct {
	db db.DataStore
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		db: utils.GetDB(),
	}
}

func (sm *SessionManager) findAuthorID(groupId string, sessionCookie *string) *string {
	if sessionCookie == nil {
		return nil
	}

	var cookie = *sessionCookie

	var replacerSession = regexp.MustCompile("^\"|\"$")

	var _ = strings.Split(replacerSession.ReplaceAllString(cookie, ""), ",")
	return nil
}

func (sm *SessionManager) getSessionInfo() {

}

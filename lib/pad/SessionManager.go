package pad

import (
	"regexp"
	"strings"

	"github.com/ether/etherpad-go/lib/db"
)

type SessionManager struct {
	db db.DataStore
}

func NewSessionManager(db db.DataStore) *SessionManager {
	return &SessionManager{
		db,
	}
}

func (sm *SessionManager) doesSessionExist(sessionID string) bool {
	//var session = sm.db.GetSession(sessionID)
	return false
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

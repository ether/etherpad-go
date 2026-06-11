package pad

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
)

// API sessions bind an author to a group for access to that group's private
// pads (createSession in the original HTTP API). Like the original — which
// stores session:*, group2sessions:* and author2sessions:* records in its
// key-value database — they live in the generic key-value storage of the
// DataStore. The lib/api/session endpoints delegate to this manager and
// SecurityManager.CheckAccess consults it via FindAuthorID.
const (
	apiSessionKeyPrefix = "apisession:"
	groupSessionsPrefix = "apisessions:group:"
	authorSessionsKey   = "apisessions:author:"
)

// ApiSessionInfo is the stored payload of an API session.
type ApiSessionInfo struct {
	GroupID    string `json:"groupID"`
	AuthorID   string `json:"authorID"`
	ValidUntil int64  `json:"validUntil"`
}

var cookieQuoteTrimmer = regexp.MustCompile(`^"|"$`)

type SessionManager struct {
	db db.DataStore
}

func NewSessionManager(db db.DataStore) *SessionManager {
	return &SessionManager{
		db,
	}
}

// CreateSession stores a new API session and registers it in the group and
// author listings. Validation of group/author existence and expiry is the
// caller's responsibility (the API layer mirrors the original's checks).
func (sm *SessionManager) CreateSession(groupID string, authorID string, validUntil int64) (string, error) {
	sessionID := "s." + utils.RandomString(16)
	encoded, err := json.Marshal(ApiSessionInfo{
		GroupID:    groupID,
		AuthorID:   authorID,
		ValidUntil: validUntil,
	})
	if err != nil {
		return "", err
	}
	if err := sm.db.SetOIDCStorageValue(apiSessionKeyPrefix+sessionID, string(encoded)); err != nil {
		return "", err
	}
	if err := sm.addToIDList(groupSessionsPrefix+groupID, sessionID); err != nil {
		return "", err
	}
	if err := sm.addToIDList(authorSessionsKey+authorID, sessionID); err != nil {
		return "", err
	}
	return sessionID, nil
}

// GetSessionInfo returns the session payload, or (nil, nil) if the session
// does not exist.
func (sm *SessionManager) GetSessionInfo(sessionID string) (*ApiSessionInfo, error) {
	payload, err := sm.db.GetOIDCStorageValue(apiSessionKeyPrefix + sessionID)
	if err != nil || payload == nil {
		return nil, err
	}
	var info ApiSessionInfo
	if err := json.Unmarshal([]byte(*payload), &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// DeleteSession removes a session and its listing entries. The boolean
// reports whether the session existed.
func (sm *SessionManager) DeleteSession(sessionID string) (bool, error) {
	info, err := sm.GetSessionInfo(sessionID)
	if err != nil {
		return false, err
	}
	if info == nil {
		return false, nil
	}
	if err := sm.db.DeleteOIDCStorageValue(apiSessionKeyPrefix + sessionID); err != nil {
		return false, err
	}
	if err := sm.removeFromIDList(groupSessionsPrefix+info.GroupID, sessionID); err != nil {
		return false, err
	}
	if err := sm.removeFromIDList(authorSessionsKey+info.AuthorID, sessionID); err != nil {
		return false, err
	}
	return true, nil
}

// ListSessionsOfGroup returns all sessions registered for a group.
func (sm *SessionManager) ListSessionsOfGroup(groupID string) (map[string]ApiSessionInfo, error) {
	return sm.listSessions(groupSessionsPrefix + groupID)
}

// ListSessionsOfAuthor returns all sessions registered for an author.
func (sm *SessionManager) ListSessionsOfAuthor(authorID string) (map[string]ApiSessionInfo, error) {
	return sm.listSessions(authorSessionsKey + authorID)
}

func (sm *SessionManager) listSessions(key string) (map[string]ApiSessionInfo, error) {
	ids, err := sm.loadIDList(key)
	if err != nil {
		return nil, err
	}
	sessions := make(map[string]ApiSessionInfo, len(ids))
	for _, id := range ids {
		info, err := sm.GetSessionInfo(id)
		if err != nil {
			return nil, err
		}
		if info != nil {
			sessions[id] = *info
		}
	}
	return sessions, nil
}

// DoesSessionExist reports whether a session record exists.
func (sm *SessionManager) DoesSessionExist(sessionID string) (bool, error) {
	info, err := sm.GetSessionInfo(sessionID)
	return info != nil, err
}

// FindAuthorID mirrors the original SessionManager.findAuthorID: the cookie
// may be enclosed in double quotes (upstream #3819) and may carry a
// comma-separated list of session ids. The author of the first session that
// belongs to the given group and is not expired is returned.
func (sm *SessionManager) FindAuthorID(groupID string, sessionCookie *string) *string {
	if sessionCookie == nil || *sessionCookie == "" {
		return nil
	}

	sessionIDs := strings.Split(cookieQuoteTrimmer.ReplaceAllString(*sessionCookie, ""), ",")
	now := time.Now().Unix()
	for _, sessionID := range sessionIDs {
		info, err := sm.GetSessionInfo(strings.TrimSpace(sessionID))
		if err != nil || info == nil {
			continue
		}
		if info.GroupID == groupID && now < info.ValidUntil {
			return &info.AuthorID
		}
	}
	return nil
}

// findAuthorID keeps the historical unexported name used by CheckAccess.
func (sm *SessionManager) findAuthorID(groupID string, sessionCookie *string) *string {
	return sm.FindAuthorID(groupID, sessionCookie)
}

func (sm *SessionManager) loadIDList(key string) ([]string, error) {
	payload, err := sm.db.GetOIDCStorageValue(key)
	if err != nil || payload == nil {
		return []string{}, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(*payload), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (sm *SessionManager) saveIDList(key string, ids []string) error {
	encoded, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return sm.db.SetOIDCStorageValue(key, string(encoded))
}

func (sm *SessionManager) addToIDList(key string, id string) error {
	ids, err := sm.loadIDList(key)
	if err != nil {
		return err
	}
	if slices.Contains(ids, id) {
		return nil
	}
	return sm.saveIDList(key, append(ids, id))
}

func (sm *SessionManager) removeFromIDList(key string, id string) error {
	ids, err := sm.loadIDList(key)
	if err != nil {
		return err
	}
	filtered := slices.DeleteFunc(ids, func(existing string) bool { return existing == id })
	return sm.saveIDList(key, filtered)
}

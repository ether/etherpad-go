// Package session implements the Etherpad API session endpoints
// (createSession, getSessionInfo, deleteSession, listSessionsOfGroup,
// listSessionsOfAuthor from the original HTTP API).
//
// API sessions bind an author to a group for access to that group's pads.
// Like the original (which stores session:*, group2sessions:* and
// author2sessions:* records in its key-value database), sessions are kept in
// the generic key-value storage of the DataStore under "apisession:" keys —
// no dedicated table is needed.
package session

import (
	"encoding/json"
	"time"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/errors"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
)

const (
	sessionKeyPrefix = "apisession:"
	groupKeyPrefix   = "apisessions:group:"
	authorKeyPrefix  = "apisessions:author:"
)

// CreateSessionRequest represents the request to create an API session
type CreateSessionRequest struct {
	GroupID    string `json:"groupID"`
	AuthorID   string `json:"authorID"`
	ValidUntil int64  `json:"validUntil"`
}

// SessionResponse represents a response with a session ID
type SessionResponse struct {
	SessionID string `json:"sessionID"`
}

// SessionInfoResponse represents the stored data of a session
type SessionInfoResponse struct {
	GroupID    string `json:"groupID"`
	AuthorID   string `json:"authorID"`
	ValidUntil int64  `json:"validUntil"`
}

// SessionWithID is a session info including its ID, used in listings
type SessionWithID struct {
	SessionID string `json:"sessionID"`
	SessionInfoResponse
}

// SessionListResponse represents a list of sessions
type SessionListResponse struct {
	Sessions []SessionWithID `json:"sessions"`
}

func loadSession(store *lib.InitStore, sessionId string) (*SessionInfoResponse, error) {
	payload, err := store.Store.GetOIDCStorageValue(sessionKeyPrefix + sessionId)
	if err != nil || payload == nil {
		return nil, err
	}
	var info SessionInfoResponse
	if err := json.Unmarshal([]byte(*payload), &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func loadIDList(store *lib.InitStore, key string) ([]string, error) {
	payload, err := store.Store.GetOIDCStorageValue(key)
	if err != nil || payload == nil {
		return []string{}, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(*payload), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func saveIDList(store *lib.InitStore, key string, ids []string) error {
	encoded, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return store.Store.SetOIDCStorageValue(key, string(encoded))
}

func addToIDList(store *lib.InitStore, key string, id string) error {
	ids, err := loadIDList(store, key)
	if err != nil {
		return err
	}
	for _, existing := range ids {
		if existing == id {
			return nil
		}
	}
	return saveIDList(store, key, append(ids, id))
}

func removeFromIDList(store *lib.InitStore, key string, id string) error {
	ids, err := loadIDList(store, key)
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(ids))
	for _, existing := range ids {
		if existing != id {
			filtered = append(filtered, existing)
		}
	}
	return saveIDList(store, key, filtered)
}

// CreateSession godoc
// @Summary Create an API session
// @Description Creates a session binding an author to a group until validUntil (unix timestamp in seconds)
// @Tags Sessions
// @Accept json
// @Produce json
// @Param request body CreateSessionRequest true "Session data"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/sessions [post]
func CreateSession(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		var request CreateSessionRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors.InvalidRequestError)
		}
		if request.GroupID == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("groupID"))
		}
		if request.AuthorID == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("authorID"))
		}
		if request.ValidUntil <= time.Now().Unix() {
			return c.Status(400).JSON(errors.NewInvalidParamError("validUntil is in the past"))
		}

		if _, err := store.Store.GetGroup(request.GroupID); err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("group does not exist"))
		}
		if _, err := store.Store.GetAuthor(request.AuthorID); err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("author does not exist"))
		}

		sessionId := "s." + utils.RandomString(16)
		info := SessionInfoResponse{
			GroupID:    request.GroupID,
			AuthorID:   request.AuthorID,
			ValidUntil: request.ValidUntil,
		}
		encoded, err := json.Marshal(info)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		if err := store.Store.SetOIDCStorageValue(sessionKeyPrefix+sessionId, string(encoded)); err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if err := addToIDList(store, groupKeyPrefix+request.GroupID, sessionId); err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if err := addToIDList(store, authorKeyPrefix+request.AuthorID, sessionId); err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		return c.JSON(SessionResponse{SessionID: sessionId})
	}
}

// GetSessionInfo godoc
// @Summary Get session info
// @Description Returns group, author and expiry of a session
// @Tags Sessions
// @Produce json
// @Param sessionId path string true "Session ID"
// @Success 200 {object} SessionInfoResponse
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/sessions/{sessionId} [get]
func GetSessionInfo(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		sessionId := c.Params("sessionId")
		info, err := loadSession(store, sessionId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if info == nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("session does not exist"))
		}
		return c.JSON(info)
	}
}

// DeleteSession godoc
// @Summary Delete a session
// @Description Deletes an API session
// @Tags Sessions
// @Produce json
// @Param sessionId path string true "Session ID"
// @Success 200 {string} string "OK"
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/sessions/{sessionId} [delete]
func DeleteSession(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		sessionId := c.Params("sessionId")
		info, err := loadSession(store, sessionId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if info == nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("session does not exist"))
		}

		if err := store.Store.DeleteOIDCStorageValue(sessionKeyPrefix + sessionId); err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if err := removeFromIDList(store, groupKeyPrefix+info.GroupID, sessionId); err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if err := removeFromIDList(store, authorKeyPrefix+info.AuthorID, sessionId); err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		return c.SendStatus(200)
	}
}

func listSessions(store *lib.InitStore, key string) ([]SessionWithID, error) {
	ids, err := loadIDList(store, key)
	if err != nil {
		return nil, err
	}
	sessions := make([]SessionWithID, 0, len(ids))
	for _, id := range ids {
		info, err := loadSession(store, id)
		if err != nil {
			return nil, err
		}
		if info == nil {
			continue
		}
		sessions = append(sessions, SessionWithID{SessionID: id, SessionInfoResponse: *info})
	}
	return sessions, nil
}

// ListSessionsOfGroup godoc
// @Summary List sessions of a group
// @Description Returns all sessions of a group
// @Tags Sessions
// @Produce json
// @Param groupId path string true "Group ID"
// @Success 200 {object} SessionListResponse
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/groups/{groupId}/sessions [get]
func ListSessionsOfGroup(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		groupId := c.Params("groupId")
		if _, err := store.Store.GetGroup(groupId); err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("group does not exist"))
		}
		sessions, err := listSessions(store, groupKeyPrefix+groupId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		return c.JSON(SessionListResponse{Sessions: sessions})
	}
}

// ListSessionsOfAuthor godoc
// @Summary List sessions of an author
// @Description Returns all sessions of an author
// @Tags Sessions
// @Produce json
// @Param authorId path string true "Author ID"
// @Success 200 {object} SessionListResponse
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/authors/{authorId}/sessions [get]
func ListSessionsOfAuthor(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		authorId := c.Params("authorId")
		if _, err := store.Store.GetAuthor(authorId); err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("author does not exist"))
		}
		sessions, err := listSessions(store, authorKeyPrefix+authorId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		return c.JSON(SessionListResponse{Sessions: sessions})
	}
}

func Init(store *lib.InitStore) {
	store.PrivateAPI.Post("/sessions", CreateSession(store))
	store.PrivateAPI.Get("/sessions/:sessionId", GetSessionInfo(store))
	store.PrivateAPI.Delete("/sessions/:sessionId", DeleteSession(store))
	store.PrivateAPI.Get("/groups/:groupId/sessions", ListSessionsOfGroup(store))
	store.PrivateAPI.Get("/authors/:authorId/sessions", ListSessionsOfAuthor(store))
}

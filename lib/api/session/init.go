// Package session implements the Etherpad API session endpoints
// (createSession, getSessionInfo, deleteSession, listSessionsOfGroup,
// listSessionsOfAuthor from the original HTTP API). Storage and lookup live
// in pad.SessionManager, which SecurityManager.CheckAccess also consults for
// access to private group pads.
package session

import (
	"sort"
	"time"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/errors"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/gofiber/fiber/v3"
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

func toInfoResponse(info pad.ApiSessionInfo) SessionInfoResponse {
	return SessionInfoResponse{
		GroupID:    info.GroupID,
		AuthorID:   info.AuthorID,
		ValidUntil: info.ValidUntil,
	}
}

func toListResponse(sessions map[string]pad.ApiSessionInfo) SessionListResponse {
	list := make([]SessionWithID, 0, len(sessions))
	for id, info := range sessions {
		list = append(list, SessionWithID{SessionID: id, SessionInfoResponse: toInfoResponse(info)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].SessionID < list[j].SessionID })
	return SessionListResponse{Sessions: list}
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
func CreateSession(store *lib.InitStore, sessions *pad.SessionManager) fiber.Handler {
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

		sessionId, err := sessions.CreateSession(request.GroupID, request.AuthorID, request.ValidUntil)
		if err != nil {
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
func GetSessionInfo(sessions *pad.SessionManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		info, err := sessions.GetSessionInfo(c.Params("sessionId"))
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if info == nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("session does not exist"))
		}
		return c.JSON(toInfoResponse(*info))
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
func DeleteSession(sessions *pad.SessionManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		deleted, err := sessions.DeleteSession(c.Params("sessionId"))
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if !deleted {
			return c.Status(404).JSON(errors.NewInvalidParamError("session does not exist"))
		}
		return c.SendStatus(200)
	}
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
func ListSessionsOfGroup(store *lib.InitStore, sessions *pad.SessionManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		groupId := c.Params("groupId")
		if _, err := store.Store.GetGroup(groupId); err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("group does not exist"))
		}
		found, err := sessions.ListSessionsOfGroup(groupId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		return c.JSON(toListResponse(found))
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
func ListSessionsOfAuthor(store *lib.InitStore, sessions *pad.SessionManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		authorId := c.Params("authorId")
		if _, err := store.Store.GetAuthor(authorId); err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("author does not exist"))
		}
		found, err := sessions.ListSessionsOfAuthor(authorId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		return c.JSON(toListResponse(found))
	}
}

func Init(store *lib.InitStore) {
	sessions := pad.NewSessionManager(store.Store)
	store.PrivateAPI.Post("/sessions", CreateSession(store, sessions))
	store.PrivateAPI.Get("/sessions/:sessionId", GetSessionInfo(sessions))
	store.PrivateAPI.Delete("/sessions/:sessionId", DeleteSession(sessions))
	store.PrivateAPI.Get("/groups/:groupId/sessions", ListSessionsOfGroup(store, sessions))
	store.PrivateAPI.Get("/authors/:authorId/sessions", ListSessionsOfAuthor(store, sessions))
}

package groups

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/errors"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
)

// GroupIDResponse represents a response with a group ID
type GroupIDResponse struct {
	GroupID string `json:"groupID"`
}

// CreateGroupPadRequest represents the request to create a group pad
type CreateGroupPadRequest struct {
	PadName  string `json:"padName"`
	Text     string `json:"text"`
	AuthorId string `json:"authorId"`
}

// CreateGroupIfNotExistsForRequest represents the request to map an external
// id to a stable group
type CreateGroupIfNotExistsForRequest struct {
	GroupMapper string `json:"groupMapper"`
}

// GroupListResponse represents a response with all group IDs
type GroupListResponse struct {
	GroupIDs []string `json:"groupIDs"`
}

// PadListResponse represents a response with the pad IDs of a group
type PadListResponse struct {
	PadIDs []string `json:"padIDs"`
}

// CreateGroup godoc
// @Summary Create a new group
// @Description Creates a new group and returns its ID
// @Tags Groups
// @Accept json
// @Produce json
// @Success 200 {object} GroupIDResponse
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/groups [post]
func CreateGroup(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		groupId := "g." + utils.RandomString(16)
		err := store.Store.SaveGroup(groupId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		return c.JSON(GroupIDResponse{
			GroupID: groupId,
		})
	}
}

// DeleteGroup godoc
// @Summary Delete a group
// @Description Deletes a group
// @Tags Groups
// @Accept json
// @Produce json
// @Param groupId path string true "Group ID"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/groups/{groupId} [delete]
func DeleteGroup(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		groupId := c.Params("groupId")
		if groupId == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("groupId"))
		}

		// Check if group exists
		_, err := store.Store.GetGroup(groupId)
		if err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("group does not exist"))
		}

		// Delete the group
		err = store.Store.RemoveGroup(groupId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		return c.SendStatus(200)
	}
}

// CreateGroupPad godoc
// @Summary Create a pad in a group
// @Description Creates a new pad within a group
// @Tags Groups
// @Accept json
// @Produce json
// @Param groupId path string true "Group ID"
// @Param request body CreateGroupPadRequest true "Pad data"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 409 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/groups/{groupId}/pads [post]
func CreateGroupPad(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		groupId := c.Params("groupId")
		if groupId == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("groupId"))
		}

		var request CreateGroupPadRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors.InvalidRequestError)
		}
		if request.PadName == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("padName"))
		}

		// Validate pad name
		if strings.ContainsAny(request.PadName, "/?&#$") {
			return c.Status(400).JSON(errors.NewInvalidParamError("malformed padName"))
		}

		// Check if group exists
		_, err := store.Store.GetGroup(groupId)
		if err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("group does not exist"))
		}

		// Create pad ID: groupId$padName
		padId := groupId + "$" + request.PadName

		// Check if pad already exists
		exists, err := store.PadManager.DoesPadExist(padId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		if exists != nil && *exists {
			return c.Status(409).JSON(errors.NewInvalidParamError("pad already exists"))
		}

		// Create the pad
		var textPtr *string
		if request.Text != "" {
			textPtr = &request.Text
		}
		var authorPtr *string
		if request.AuthorId != "" {
			authorPtr = &request.AuthorId
		}

		_, err = store.PadManager.GetPad(padId, textPtr, authorPtr)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		return c.JSON(fiber.Map{
			"padID": padId,
		})
	}
}

// CreateGroupIfNotExistsFor godoc
// @Summary Get or create a group for an external mapper
// @Description Returns a stable group for the given external mapper id, creating it if necessary. The group id is derived deterministically from the mapper, so the same mapper always maps to the same group.
// @Tags Groups
// @Accept json
// @Produce json
// @Param request body CreateGroupIfNotExistsForRequest true "External mapper id"
// @Success 200 {object} GroupIDResponse
// @Failure 400 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/groups/createIfNotExistsFor [post]
func CreateGroupIfNotExistsFor(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		var request CreateGroupIfNotExistsForRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors.InvalidRequestError)
		}
		if request.GroupMapper == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("groupMapper"))
		}

		// Unlike the original (which stores a mapper2group record), the group
		// id is derived deterministically from the mapper, which makes the
		// operation idempotent without extra storage.
		sum := sha256.Sum256([]byte(request.GroupMapper))
		groupId := "g." + hex.EncodeToString(sum[:])[:16]

		// SaveGroup is an upsert on every backend, so this is idempotent.
		if err := store.Store.SaveGroup(groupId); err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		return c.JSON(GroupIDResponse{GroupID: groupId})
	}
}

// ListAllGroups godoc
// @Summary List all groups
// @Description Returns the IDs of all existing groups
// @Tags Groups
// @Produce json
// @Success 200 {object} GroupListResponse
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/groups [get]
func ListAllGroups(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		groupIds, err := store.Store.GetGroups()
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		return c.JSON(GroupListResponse{GroupIDs: *groupIds})
	}
}

// ListGroupPads godoc
// @Summary List pads of a group
// @Description Returns the IDs of all pads belonging to a group
// @Tags Groups
// @Produce json
// @Param groupId path string true "Group ID"
// @Success 200 {object} PadListResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/groups/{groupId}/pads [get]
func ListGroupPads(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		groupId := c.Params("groupId")
		if groupId == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("groupId"))
		}

		if _, err := store.Store.GetGroup(groupId); err != nil {
			return c.Status(404).JSON(errors.NewInvalidParamError("group does not exist"))
		}

		// Group membership is derived from the `groupId$padName` id prefix.
		padIds, err := store.Store.GetPadIds()
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		groupPads := make([]string, 0)
		if padIds != nil {
			for _, padId := range *padIds {
				if strings.HasPrefix(padId, groupId+"$") {
					groupPads = append(groupPads, padId)
				}
			}
		}

		return c.JSON(PadListResponse{PadIDs: groupPads})
	}
}

func Init(store *lib.InitStore) {
	// Group management (specific paths before :groupId)
	store.PrivateAPI.Post("/groups/createIfNotExistsFor", CreateGroupIfNotExistsFor(store))
	store.PrivateAPI.Get("/groups", ListAllGroups(store))
	store.PrivateAPI.Post("/groups", CreateGroup(store))
	store.PrivateAPI.Delete("/groups/:groupId", DeleteGroup(store))

	// Group pads
	store.PrivateAPI.Get("/groups/:groupId/pads", ListGroupPads(store))
	store.PrivateAPI.Post("/groups/:groupId/pads", CreateGroupPad(store))
}

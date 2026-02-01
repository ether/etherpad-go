package groups

import (
	"strings"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/errors"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v2"
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
	return func(c *fiber.Ctx) error {
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
	return func(c *fiber.Ctx) error {
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
	return func(c *fiber.Ctx) error {
		groupId := c.Params("groupId")
		if groupId == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("groupId"))
		}

		var request CreateGroupPadRequest
		if err := c.BodyParser(&request); err != nil {
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

func Init(store *lib.InitStore) {
	// Group management
	store.PrivateAPI.Post("/groups", CreateGroup(store))
	store.PrivateAPI.Delete("/groups/:groupId", DeleteGroup(store))

	// Group pads
	store.PrivateAPI.Post("/groups/:groupId/pads", CreateGroupPad(store))
}

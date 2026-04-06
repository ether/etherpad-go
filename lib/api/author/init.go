package author

import (
	"encoding/json"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/errors"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/gofiber/fiber/v3"
)

// CreateDto represents the request to create an author
type CreateDto struct {
	Name string `json:"name" validate:"required" example:"John Doe"`
}

// CreateDtoResponse represents the response when creating an author
type CreateDtoResponse struct {
	AuthorId string `json:"authorId" example:"a.s8oes9dhwrvt0zif"`
}

// CreateAuthorIfNotExistsForRequest represents the request to create an author if not exists
type CreateAuthorIfNotExistsForRequest struct {
	AuthorMapper string `json:"authorMapper" validate:"required"`
	Name         string `json:"name"`
}

// AuthorNameResponse represents the response with author name
type AuthorNameResponse struct {
	AuthorName string `json:"authorName"`
}

// PadsResponse represents a list of pad IDs
type PadsResponse struct {
	PadIds []string `json:"padIds"`
}

// CreateAuthor godoc
// @Summary Create a new author
// @Description Creates a new author with the specified name
// @Tags Authors
// @Accept json
// @Produce json
// @Param author body CreateDto true "Author data"
// @Success 200 {object} CreateDtoResponse
// @Failure 400 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/author [post]
func CreateAuthor(initStore *lib.InitStore, authorManager *author.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		var dto CreateDto
		err := json.Unmarshal(c.Body(), &dto)
		if err != nil {
			return c.Status(400).JSON(errors.InvalidRequestError)
		}

		// Validate required fields
		if err := initStore.Validator.Struct(dto); err != nil {
			return c.Status(400).JSON(errors.NewInvalidParamError(err.Error()))
		}

		createdAuthor, err := authorManager.CreateAuthor(&dto.Name)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		return c.JSON(CreateDtoResponse{
			AuthorId: createdAuthor.Id,
		})
	}
}

// CreateAuthorIfNotExistsFor godoc
// @Summary Create an author if not exists for a mapper
// @Description Creates an author for a mapper if it doesn't exist, otherwise returns existing author ID
// @Tags Authors
// @Accept json
// @Produce json
// @Param request body CreateAuthorIfNotExistsForRequest true "Author mapper and name"
// @Success 200 {object} CreateDtoResponse
// @Failure 400 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/author/createIfNotExistsFor [post]
func CreateAuthorIfNotExistsFor(initStore *lib.InitStore, authorManager *author.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		var request CreateAuthorIfNotExistsForRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors.InvalidRequestError)
		}
		if request.AuthorMapper == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("authorMapper"))
		}

		// Get or create author by token (mapper)
		existingAuthor, err := authorManager.GetAuthor4Token(request.AuthorMapper)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		// Update name if provided
		if request.Name != "" {
			authorManager.SetAuthorName(existingAuthor.Id, request.Name)
		}

		return c.JSON(CreateDtoResponse{
			AuthorId: existingAuthor.Id,
		})
	}
}

// GetAuthorName godoc
// @Summary Get author name
// @Description Returns the name of an author
// @Tags Authors
// @Accept json
// @Produce json
// @Param authorId path string true "Author ID"
// @Success 200 {object} AuthorNameResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/author/{authorId}/name [get]
func GetAuthorName(initStore *lib.InitStore, authorManager *author.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		authorId := c.Params("authorId")
		if authorId == "" {
			return c.Status(400).JSON(errors.NewInvalidParamError("authorId is required"))
		}

		foundAuthor, err := authorManager.GetAuthor(authorId)
		if foundAuthor == nil || err != nil {
			return c.Status(404).JSON(errors.AuthorNotFoundError)
		}

		authorName := ""
		if foundAuthor.Name != nil {
			authorName = *foundAuthor.Name
		}

		return c.JSON(AuthorNameResponse{
			AuthorName: authorName,
		})
	}
}

// GetAuthor godoc
// @Summary Get an author
// @Description Returns the information of an author by their ID
// @Tags Authors
// @Accept json
// @Produce json
// @Param authorId path string true "Author ID"
// @Success 200 {object} author.Author
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/author/{authorId} [get]
func GetAuthor(initStore *lib.InitStore, authorManager *author.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		var authorId = c.Params("authorId")
		if authorId == "" {
			return c.Status(400).JSON(errors.NewInvalidParamError("authorId is required"))
		}
		var foundAuthor, err = authorManager.GetAuthor(authorId)
		if foundAuthor == nil {
			return c.Status(404).JSON(errors.AuthorNotFoundError)
		}

		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		return c.JSON(foundAuthor)
	}
}

// GetAuthorPads godoc
// @Summary Get all pads of an author
// @Description Returns all pad IDs that an author has contributed to
// @Tags Authors
// @Accept json
// @Produce json
// @Param authorId path string true "Author ID"
// @Success 200 {array} string
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/author/{authorId}/pads [get]
func GetAuthorPads(initStore *lib.InitStore, authorManager *author.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		var authorId = c.Params("authorId")
		if authorId == "" {
			return c.Status(400).JSON(errors.NewInvalidParamError("authorId is required"))
		}
		var foundAuthor, err = authorManager.GetAuthor(authorId)
		if foundAuthor == nil {
			return c.Status(404).JSON(errors.AuthorNotFoundError)
		}

		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		padsOfAuthor, err := authorManager.GetPadsOfAuthor(authorId)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}

		return c.JSON(*padsOfAuthor)
	}
}

func Init(initStore *lib.InitStore) {
	var authorManager = author.NewManager(initStore.Store)

	initStore.PrivateAPI.Post("/author", CreateAuthor(initStore, authorManager))
	initStore.PrivateAPI.Post("/author/createIfNotExistsFor", CreateAuthorIfNotExistsFor(initStore, authorManager))
	initStore.PrivateAPI.Get("/author/:authorId", GetAuthor(initStore, authorManager))
	initStore.PrivateAPI.Get("/author/:authorId/name", GetAuthorName(initStore, authorManager))
	initStore.PrivateAPI.Get("/author/:authorId/pads", GetAuthorPads(initStore, authorManager))
}

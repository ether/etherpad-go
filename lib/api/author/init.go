package author

import (
	"encoding/json"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/errors"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/gofiber/fiber/v2"
)

type CreateDto struct {
	Name string `json:"name" validate:"required"`
}

type CreateDtoResponse struct {
	AuthorId string `json:"authorId"`
}

func Init(initStore *lib.InitStore) {
	var authorManager = author.NewManager(initStore.Store)

	initStore.C.Post("/author", func(c *fiber.Ctx) error {
		var dto CreateDto
		err := json.Unmarshal(c.Body(), &dto)
		if err != nil {
			return c.Status(400).JSON(errors.InvalidRequestError)
		}
		err = initStore.Validator.Struct(dto)
		if err != nil {
			return c.Status(400).JSON(errors.NewInvalidParamError(err.Error()))
		}

		createdAuthor, err := authorManager.CreateAuthor(&dto.Name)
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		return c.JSON(CreateDtoResponse{
			AuthorId: createdAuthor.Id,
		})
	})

	initStore.C.Get("/author/:authorId", func(c *fiber.Ctx) error {
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
	})
	initStore.C.Get("/author/:authorId/pads", func(c *fiber.Ctx) error {
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

		return c.JSON(foundAuthor.PadIDs)
	})
}

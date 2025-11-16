package author

import (
	"encoding/json"

	error2 "github.com/ether/etherpad-go/lib/api/error"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type CreateDto struct {
	Name string `json:"name" validate:"required"`
}

type CreateDtoResponse struct {
	AuthorId string `json:"authorId"`
}

func Init(c *fiber.App, db db.DataStore, validator *validator.Validate) {
	var authorManager = author.NewManager(db)

	c.Post("/author", func(c *fiber.Ctx) error {
		var dto CreateDto
		err := json.Unmarshal(c.Body(), &dto)
		if err != nil {
			return c.Status(400).JSON(error2.Error{
				Message: "Invalid request " + err.Error(),
				Error:   400,
			})
		}
		err = validator.Struct(dto)
		if err != nil {
			return c.Status(400).JSON(error2.Error{
				Message: "Validation error: " + err.Error(),
			})
		}

		var createdAuthor = authorManager.CreateAuthor(&dto.Name)
		return c.JSON(CreateDtoResponse{
			AuthorId: createdAuthor.Id,
		})
	})

	c.Get("/author/:authorId", func(c *fiber.Ctx) error {
		var authorId = c.Params("authorId")
		if authorId == "" {
			return c.Status(400).JSON(error2.Error{
				Message: "authorId is required",
				Error:   400,
			})
		}
		var foundAuthor, err = authorManager.GetAuthor(authorId)
		if foundAuthor == nil {
			return c.Status(404).JSON(error2.Error{
				Message: "Author not found",
				Error:   404,
			})
		}

		if err != nil {
			return c.Status(500).JSON(error2.Error{
				Message: "Internal server error",
				Error:   500,
			})
		}

		return c.JSON(foundAuthor)
	})
	c.Get("/author/:authorId/pads", func(c *fiber.Ctx) error {
		var authorId = c.Params("authorId")
		if authorId == "" {
			return c.Status(400).JSON(error2.Error{
				Message: "authorId is required",
				Error:   400,
			})
		}
		var foundAuthor, err = authorManager.GetAuthor(authorId)
		if foundAuthor == nil {
			return c.Status(404).JSON(error2.Error{
				Message: "Author not found",
				Error:   404,
			})
		}

		if err != nil {
			return c.Status(500).JSON(error2.Error{
				Message: "Internal server error",
				Error:   500,
			})
		}

		return c.JSON(foundAuthor.PadIDs)
	})
}

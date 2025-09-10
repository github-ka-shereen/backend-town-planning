package controllers

import (
	"github.com/gofiber/fiber/v2"
)

func (c *SearchController) SearchUsersController(ctx *fiber.Ctx) error {
	query := ctx.Query("q")
	if query == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Search query is required",
		})
	}

	results, err := c.repo.SearchUsers(query)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Search failed",
		})
	}

	var matches []interface{}
	for _, hit := range results.Hits {
		doc, err := c.repo.GetUserDocument(hit.ID)
		if err != nil {
			continue // or log error
		}
		matches = append(matches, doc)
	}

	return ctx.JSON(fiber.Map{
		"results": matches,
		"total":   results.Total,
	})
}

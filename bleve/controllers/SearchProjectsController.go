// controllers/search_controller.go
package controllers

import (
	"github.com/gofiber/fiber/v2"
)

func (c *SearchController) SearchProjectsController(ctx *fiber.Ctx) error {
	query := ctx.Query("q")
	city := ctx.Query("city")

	// Perform the search
	results, err := c.repo.SearchProjects(query, city)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Project search failed",
		})
	}

	// You may choose to enrich results from Bleve hits here
	var matches []interface{}
	for _, hit := range results.Hits {
		doc, err := c.repo.GetProjectDocument(hit.ID)
		if err != nil {
			continue // optionally log the error
		}
		matches = append(matches, doc)
	}

	return ctx.JSON(fiber.Map{
		"results": matches,
		"total":   results.Total,
	})
}

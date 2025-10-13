package controllers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (c *SearchController) SearchStandsController(ctx *fiber.Ctx) error {
	query := ctx.Query("q")
	status := ctx.Query("status")
	standType := ctx.Query("stand_type")
	standCurrency := ctx.Query("stand_currency")

	// Optional boolean filter
	activeStr := ctx.Query("active")
	var active *bool

	if activeStr != "" {
		val, err := strconv.ParseBool(activeStr)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid 'active' value",
			})
		}
		active = &val
	}

	// Perform the search
	results, err := c.repo.SearchStands(query, status, standType, active, standCurrency)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Search failed",
		})
	}

	// You may choose to enrich results from Bleve hits here
	var matches []interface{}
	for _, hit := range results.Hits {
		doc, err := c.repo.GetStandDocument(hit.ID)
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

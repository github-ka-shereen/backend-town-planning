// controllers/search_controller.go
package controllers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (c *SearchController) SearchVATRatesController(ctx *fiber.Ctx) error {
	activeStr := ctx.Query("is_active")
	usedStr := ctx.Query("used")
	sortStr := ctx.Query("sort")

	var active, used *bool
	var err error

	if activeStr != "" {
		val, err := strconv.ParseBool(activeStr)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid 'active' value",
			})
		}
		active = &val
	}

	if usedStr != "" {
		val, err := strconv.ParseBool(usedStr)
		if err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid 'used' value",
			})
		}
		used = &val
	}

	// Parse sort parameter
	var sortBy []string
	if sortStr != "" {
		// Split by comma for multiple sort fields
		sortBy = strings.Split(sortStr, ",")
		// Trim whitespace from each sort field
		for i, field := range sortBy {
			sortBy[i] = strings.TrimSpace(field)
		}
	}

	// Perform the search
	results, err := c.repo.SearchVATRates(active, used, sortBy)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "VAT rate search failed",
		})
	}

	// Transform the results to match your preferred structure
	var matches []map[string]interface{}
	for _, hit := range results.Hits {
		doc, err := c.repo.GetVATRateDocument(hit.ID)
		if err != nil {
			continue // optionally log the error
		}
		matches = append(matches, doc.(map[string]interface{}))
	}

	return ctx.JSON(fiber.Map{
		"results": matches,
		"total":   results.Total,
	})
}

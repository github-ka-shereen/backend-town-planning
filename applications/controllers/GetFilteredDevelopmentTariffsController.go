package controllers

import (
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// GetFilteredTariffsController handles the fetching of filtered tariffs
func (ac *ApplicationController) GetFilteredDevelopmentTariffsController(c *fiber.Ctx) error {
	// Parse query parameters
	pageSize := c.QueryInt("page_size", 10) // Default to 10 if not provided
	if pageSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid page_size parameter",
			"error":   "page_size must be greater than 0",
		})
	}

	page := c.QueryInt("page", 1) // Default to page 1 if not provided
	if page <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid page parameter",
			"error":   "page must be greater than 0",
		})
	}

	// Parse optional filters
	developmentCategoryID := c.Query("development_category_id")
	isActive := c.Query("is_active")

	// Calculate offset for pagination
	offset := (page - 1) * pageSize

	// Build filters map
	filters := make(map[string]string)
	if developmentCategoryID != "" {
		filters["development_category_id"] = developmentCategoryID
	}
	if isActive != "" {
		filters["is_active"] = isActive
	}

	// Fetch filtered tariffs from the repository
	tariffs, total, err := ac.ApplicationRepo.GetFilteredDevelopmentTariffs(pageSize, offset, filters)
	if err != nil {
		config.Logger.Error("Failed to fetch filtered tariffs", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch tariffs",
			"error":   err.Error(),
		})
	}

	// Calculate total pages
	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	config.Logger.Info("Successfully fetched filtered tariffs",
		zap.Int("page", page),
		zap.Int("pageSize", pageSize),
		zap.Int64("total", total),
		zap.Int("resultsCount", len(tariffs)))

	// Return paginated response
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Tariffs fetched successfully",
		"data": fiber.Map{
			"data": tariffs,
			"meta": fiber.Map{
				"current_page": page,
				"page_size":    pageSize,
				"total":        total,
				"total_pages":  totalPages,
			},
		},
	})
}
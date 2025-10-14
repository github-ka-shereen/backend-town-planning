package controllers

import (
	"strings"
	"town-planning-backend/config"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// DefaultFilters for Development Categories
var DefaultDevelopmentCategoryFilters = map[string]string{
	"is_active": "true",
}

// GetAllDevelopmentCategories handles the API request for fetching development categories with filtering and pagination
func (ac *ApplicationController) GetAllDevelopmentCategories(c *fiber.Ctx) error {
	// Parse pagination parameters
	pageSize := c.QueryInt("page_size", 10) // Default to 10 items per page
	if pageSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid page_size parameter",
			"error":   "page_size must be greater than 0",
		})
	}

	page := c.QueryInt("page", 1)
	if page <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid page parameter",
			"error":   "page must be greater than 0",
		})
	}

	// Helper function to clean query parameters
	cleanQueryParam := func(param string) string {
		param = strings.TrimSpace(param)
		if param == "" || strings.ToLower(param) == "null" {
			return ""
		}
		return param
	}

	// Extract and clean query parameters
	isActive := cleanQueryParam(c.Query("is_active"))
	isSystem := cleanQueryParam(c.Query("is_system"))
	createdBy := cleanQueryParam(c.Query("created_by"))

	offset := (page - 1) * pageSize
	filters := make(map[string]string)

	// Build filters
	if isActive != "" {
		filters["is_active"] = isActive
	}
	if isSystem != "" {
		filters["is_system"] = isSystem
	}
	if createdBy != "" {
		filters["created_by"] = createdBy
	}

	// Fetch paginated results
	paginatedCategories, total, err := ac.ApplicationRepo.GetFilteredDevelopmentCategories(pageSize, offset, filters)
	if err != nil {
		config.Logger.Error("Failed to fetch paginated development categories", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch development categories",
			"error":   err.Error(),
		})
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)
	
	response := fiber.Map{
		"success": true,
		"message": "Development categories fetched successfully",
		"data": fiber.Map{
			"data": paginatedCategories,
			"meta": fiber.Map{
				"current_page": page,
				"page_size":    pageSize,
				"total":        total,
				"total_pages":  totalPages,
			},
		},
	}

	// Log filtering information
	isDefault := utils.IsDefaultFilter(filters, DefaultDevelopmentCategoryFilters)
	config.Logger.Info("Development Category filters", 
		zap.Any("filters", filters),
		zap.Bool("is_default", isDefault),
	)

	return c.Status(fiber.StatusOK).JSON(response)
}
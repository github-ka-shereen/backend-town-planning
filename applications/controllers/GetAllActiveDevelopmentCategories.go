package controllers

import (
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (ac *ApplicationController) GetAllActiveDevelopmentCategories(c *fiber.Ctx) error {
	// Parse optional is_active query parameter
	var isActive *bool

	// Check if is_active parameter is provided
	if isActiveParam := c.Query("is_active"); isActiveParam != "" {
		isActiveValue := isActiveParam == "true"
		isActive = &isActiveValue
	} else {
		// Default to true if not specified
		defaultValue := true
		isActive = &defaultValue
	}

	// Fetch all development categories
	categories, err := ac.ApplicationRepo.GetAllDevelopmentCategories(isActive)
	if err != nil {
		config.Logger.Error("Failed to fetch development categories",
			zap.Error(err),
			zap.Any("is_active_filter", isActive),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch development categories",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Development categories fetched successfully",
		zap.Int("count", len(categories)),
		zap.Bool("is_active", *isActive),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Development categories fetched successfully",
		"data":    categories,
	})
}

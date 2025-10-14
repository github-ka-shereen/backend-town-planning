package controllers

import (
	"town-planning-backend/db/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CreateDevelopmentCategoryRequest struct {
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description"`
	CreatedBy   string  `json:"created_by" validate:"required"`
	IsSystem    bool    `json:"is_system"`
	IsActive    bool    `json:"is_active"`
}

func (ac *ApplicationController) CreateDevelopmentCategory(c *fiber.Ctx) error {
	var req CreateDevelopmentCategoryRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Check if category with same name already exists
	existingCategory, err := ac.ApplicationRepo.GetDevelopmentCategoryByName(req.Name)
	if err != nil && err != gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to check existing category",
			"error":   err.Error(),
		})
	}

	if existingCategory != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"message": "Development category with this name already exists",
			"error":   "duplicate_name",
		})
	}

	// Create new development category
	newCategory := models.DevelopmentCategory{
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    req.IsSystem,
		IsActive:    req.IsActive,
		CreatedBy:   req.CreatedBy,
	}

	createdCategory, err := ac.ApplicationRepo.CreateDevelopmentCategory(&newCategory)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create development category",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Development category created successfully",
		"data":    createdCategory,
	})
}

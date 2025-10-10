package controllers

import (
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AddStandTypeRequest represents the request body for adding a stand type
type AddStandTypeRequest struct {
	Name        string  `json:"name" validate:"required,min=2,max=100"`
	Description *string `json:"description" validate:"max=500"`
	CreatedBy   string  `json:"created_by" validate:"required"`
}

func (sc *StandController) AddStandTypesController(c *fiber.Ctx) error {
	var req AddStandTypeRequest

	// Parse and validate request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// --- Start Database Transaction ---
	tx := sc.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not start database transaction",
			"error":   tx.Error.Error(),
		})
	}

	// Defer rollback
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during stand type creation", zap.Any("panic", r))
			panic(r)
		}
	}()

	// Create the stand type
	standType := models.StandType{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    false, // User-created stand types are not system types
		IsActive:    true,  // Default to active
		CreatedBy:   req.CreatedBy,
	}

	createdStandType, err := sc.StandRepo.AddStandTypes(tx, &standType)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to create stand type", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not create stand type",
			"error":   err.Error(),
		})
	}

	// --- Commit Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to finalize stand type creation",
			"error":   err.Error(),
		})
	}

	// --- Success ---
	config.Logger.Info("Stand type created successfully",
		zap.String("standTypeID", standType.ID.String()),
		zap.String("standTypeName", standType.Name))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Stand type created successfully",
		"stand_type": fiber.Map{
			"data": createdStandType,
		},
	})
}

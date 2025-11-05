package controllers

import (
	"net/http"
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (dc *DocumentController) CreateDocument(c *fiber.Ctx) error {
	config.Logger.Info("Starting transaction for document creation")

	// Start transaction
	tx := dc.DB.Session(&gorm.Session{}).WithContext(c.Context()).Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to start transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	txCommitted := false
	defer func() {
		if !txCommitted {
			tx.Rollback()
			config.Logger.Warn("Transaction rolled back")
		}
	}()

	// Call service and pass the transaction
	response, err := dc.DocumentService.UnifiedCreateDocument(tx, c, nil, nil, nil)
	if err != nil {
		config.Logger.Error("Document creation failed", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Commit transaction if all went well
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to commit transaction",
			"error":   err.Error(),
		})
	}
	txCommitted = true
	config.Logger.Info("Transaction committed successfully")

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"data": response,
	})
}

// controllers/application_controller.go
package controllers

import (
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateTariffRequest represents the request payload for creating a tariff
type CreateTariffRequest struct {
	DevelopmentCategoryID  string          `json:"development_category_id" validate:"required,uuid4"`
	PricePerSquareMeter    decimal.Decimal `json:"price_per_square_meter" validate:"required,min=0"`
	PermitFee              decimal.Decimal `json:"permit_fee" validate:"required,min=0"`
	InspectionFee          decimal.Decimal `json:"inspection_fee" validate:"required,min=0"`
	Currency               string          `json:"currency" validate:"required"`
	DevelopmentLevyPercent decimal.Decimal `json:"development_levy_percent" validate:"required,min=0,max=100"`
	IsActive               bool            `json:"is_active"`
	CreatedBy              string          `json:"created_by" validate:"required,email"`
}

// CreateNewTariff handles the creation of a new tariff
func (ac *ApplicationController) CreateNewTariff(c *fiber.Ctx) error {
	var req CreateTariffRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Invalid request body for CreateNewTariff", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Start transaction
	config.Logger.Info("Starting transaction for tariff creation")
	tx := ac.DB.Session(&gorm.Session{}).WithContext(c.Context()).Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to start transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	// Defer transaction rollback/commit handling
	txCommitted := false
	defer func() {
		if !txCommitted && tx != nil {
			tx.Rollback()
			config.Logger.Warn("Transaction rolled back due to error")
		}
	}()

	// Check if development category exists within transaction
	var developmentCategory models.DevelopmentCategory
	if err := tx.Where("id = ? AND is_active = ?", req.DevelopmentCategoryID, true).First(&developmentCategory).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			config.Logger.Error("Development category not found or inactive",
				zap.String("categoryID", req.DevelopmentCategoryID))
			tx.Rollback()
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Development category not found or inactive",
				"error":   "invalid_development_category",
			})
		}
		config.Logger.Error("Failed to verify development category", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to verify development category",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Development category verified",
		zap.String("categoryID", developmentCategory.ID.String()),
		zap.String("categoryName", developmentCategory.Name))

	var updatedOldTariff *models.Tariff

	// If creating an active tariff, deactivate any existing active tariff for this category within transaction
	if req.IsActive {
		activeTariff, err := ac.getActiveTariffForCategory(tx, req.DevelopmentCategoryID)
		if err != nil {
			config.Logger.Error("Failed to get active tariff", zap.Error(err))
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to check active tariff",
				"error":   err.Error(),
			})
		}

		if activeTariff != nil {
			config.Logger.Info("Deactivating existing active tariff",
				zap.String("tariffID", activeTariff.ID.String()),
				zap.String("category", developmentCategory.Name))

			updatedOldTariff, err = ac.deactivateTariff(tx, activeTariff.ID, req.CreatedBy)
			if err != nil {
				config.Logger.Error("Failed to deactivate old tariff",
					zap.Error(err),
					zap.String("oldTariffID", activeTariff.ID.String()))
				tx.Rollback()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": "Failed to deactivate old tariff",
					"error":   err.Error(),
				})
			}
			config.Logger.Info("Successfully deactivated old tariff")
		}
	}

	// Prepare the new tariff for creation
	newTariff := models.Tariff{
		ID:                     uuid.New(),
		DevelopmentCategoryID:  developmentCategory.ID,
		PricePerSquareMeter:    req.PricePerSquareMeter,
		PermitFee:              req.PermitFee,
		InspectionFee:          req.InspectionFee,
		Currency:               req.Currency,
		DevelopmentLevyPercent: req.DevelopmentLevyPercent,
		ValidFrom:              time.Now(),
		ValidTo:                nil, // NULL means currently active
		IsActive:               req.IsActive,
	}

	// Set audit fields
	now := time.Now()
	newTariff.CreatedAt = now
	newTariff.UpdatedAt = now

	// Call the repository to create the new tariff within the transaction
	createdTariff, err := ac.createTariff(tx, &newTariff)
	if err != nil {
		config.Logger.Error("Failed to create new tariff", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create tariff",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Successfully created new tariff",
		zap.String("tariffID", createdTariff.ID.String()),
		zap.String("category", developmentCategory.Name),
		zap.String("pricePerSqM", createdTariff.PricePerSquareMeter.String()))

	// Commit the transaction after all operations succeed
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize tariff creation",
			"error":   err.Error(),
		})
	}
	txCommitted = true
	config.Logger.Info("Transaction committed successfully")

	// Return both the old (updated) and new tariffs
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Tariff created successfully",
		"data": fiber.Map{
			"old_tariff": updatedOldTariff, // This will be nil if no existing tariff was updated
			"new_tariff": createdTariff,
		},
	})
}

// Helper method to get active tariff for category within transaction
func (ac *ApplicationController) getActiveTariffForCategory(tx *gorm.DB, developmentCategoryID string) (*models.Tariff, error) {
	var tariff models.Tariff

	now := time.Now()
	err := tx.Where("development_category_id = ? AND is_active = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)",
		developmentCategoryID, true, now, now).
		Order("valid_from DESC").
		First(&tariff).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &tariff, nil
}

// Helper method to deactivate tariff within transaction
func (ac *ApplicationController) deactivateTariff(tx *gorm.DB, tariffID uuid.UUID, updatedBy string) (*models.Tariff, error) {
	var tariff models.Tariff

	if err := tx.Where("id = ?", tariffID).First(&tariff).Error; err != nil {
		return nil, err
	}

	tariff.IsActive = false
	tariff.UpdatedAt = time.Now()

	if err := tx.Save(&tariff).Error; err != nil {
		return nil, err
	}

	// Preload the development category for the response
	if err := tx.Preload("DevelopmentCategory").First(&tariff, tariff.ID).Error; err != nil {
		return nil, err
	}

	return &tariff, nil
}

// Helper method to create tariff within transaction
func (ac *ApplicationController) createTariff(tx *gorm.DB, tariff *models.Tariff) (*models.Tariff, error) {
	if err := tx.Create(tariff).Error; err != nil {
		return nil, err
	}

	// Preload the development category relationship
	if err := tx.Preload("DevelopmentCategory").First(tariff, tariff.ID).Error; err != nil {
		return nil, err
	}

	return tariff, nil
}

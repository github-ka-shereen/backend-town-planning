package controllers

import (
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateVATRateController handles the creation of a new VAT rate.
func (pc *ApplicantController) CreateVATRateController(c *fiber.Ctx) error {
	var vatRate models.VATRate
	if err := c.BodyParser(&vatRate); err != nil {
		config.Logger.Error("Invalid request body for CreateVATRateController", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Start transaction
	config.Logger.Info("Starting transaction for VAT rate creation")
	tx := pc.DB.Session(&gorm.Session{}).WithContext(c.Context()).Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to start transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start transaction",
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

	// Get the currently active VAT rate within transaction
	activeRate, err := pc.ApplicantRepo.GetActiveVATRate(tx)
	if err != nil {
		config.Logger.Error("Failed to get active VAT rate", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check existing VAT rate",
		})
	}

	var updatedOldRate *models.VATRate

	// If an active rate exists, deactivate it within the same transaction
	if activeRate != nil {
		config.Logger.Info("Deactivating existing active VAT rate",
			zap.String("rateID", activeRate.ID.String()),
			zap.String("rate", activeRate.Rate.String()))

		updatedOldRate, err = pc.ApplicantRepo.DeactivateVATRate(tx, activeRate.ID, vatRate.CreatedBy)
		if err != nil {
			config.Logger.Error("Failed to deactivate old VAT rate",
				zap.Error(err),
				zap.String("oldRateID", activeRate.ID.String()))
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to deactivate old VAT rate",
			})
		}
		config.Logger.Info("Successfully deactivated old VAT rate")
	}

	// Prepare the new VAT rate for creation
	vatRate.ID = uuid.New()
	now := time.Now()

	// If ValidFrom was not set in the request, default it to now
	if vatRate.ValidFrom.IsZero() {
		vatRate.ValidFrom = now
	}

	vatRate.IsActive = true // New rate is active
	vatRate.CreatedAt = now
	vatRate.Used = false // Newly created rate is not yet used in a transaction
	vatRate.UpdatedAt = now

	// Call the repository to create the new VAT rate within the transaction
	createdRate, err := pc.ApplicantRepo.CreateVATRate(tx, &vatRate)
	if err != nil {
		config.Logger.Error("Failed to create new VAT rate", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create VAT rate",
		})
	}
	config.Logger.Info("Successfully created new VAT rate",
		zap.String("rateID", createdRate.ID.String()),
		zap.String("rate", createdRate.Rate.String()))

	// Index both the created rate and updated (old) rate within the transaction
	if pc.BleveRepo != nil {
		// Index the newly created rate within transaction
		err := pc.BleveRepo.IndexSingleVATRate(*createdRate)
		if err != nil {
			config.Logger.Error("Error indexing new VAT rate within transaction",
				zap.Error(err),
				zap.String("vatRateID", createdRate.ID.String()))
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to index new VAT rate",
			})
		} else {
			config.Logger.Info("Successfully indexed new VAT rate within transaction",
				zap.String("vatRateID", createdRate.ID.String()))
		}

		// Index the updated (old) rate if it exists within transaction
		if updatedOldRate != nil {
			err := pc.BleveRepo.IndexSingleVATRate(*updatedOldRate)
			if err != nil {
				config.Logger.Error("Error indexing updated old VAT rate within transaction",
					zap.Error(err),
					zap.String("oldVatRateID", updatedOldRate.ID.String()))
				tx.Rollback()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to index old VAT rate",
				})
			} else {
				config.Logger.Info("Successfully indexed updated old VAT rate within transaction",
					zap.String("oldVatRateID", updatedOldRate.ID.String()))
			}
		}
	} else {
		config.Logger.Warn("IndexingService is nil, skipping document indexing for VAT rate")
	}

	// Commit the transaction after all operations succeed
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to finalize VAT rate creation",
		})
	}
	txCommitted = true
	config.Logger.Info("Transaction committed successfully")

	// Return both the old (updated) and new VAT rates
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"old_vat_rate": updatedOldRate, // This will be nil if no existing rate was updated
		"new_vat_rate": createdRate,
	})
}

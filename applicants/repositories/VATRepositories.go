package repositories

import (
	"fmt"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// GetActiveVATRate retrieves the currently active VAT rate
func (ar *applicantRepository) GetActiveVATRate(tx *gorm.DB) (*models.VATRate, error) {
	var vatRate models.VATRate

	// Get the rate where IsActive is true and ValidTo is either null or in the future
	err := tx.Where("is_active = ? AND (valid_to IS NULL OR valid_to > ?)", true, time.Now()).
		First(&vatRate).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No active rate found, return nil
		}
		config.Logger.Error("Failed to get active VAT rate", zap.Error(err))
		return nil, fmt.Errorf("failed to get active VAT rate: %w", err)
	}

	return &vatRate, nil
}

// CreateVATRate creates a new VAT rate
func (ar *applicantRepository) CreateVATRate(tx *gorm.DB, vatRate *models.VATRate) (*models.VATRate, error) {
	if err := tx.Create(vatRate).Error; err != nil {
		config.Logger.Error("Failed to create VAT rate",
			zap.Error(err),
			zap.String("rate", vatRate.Rate.String()))
		return nil, fmt.Errorf("failed to create VAT rate: %w", err)
	}

	return vatRate, nil
}

// DeactivateVATRate deactivates an existing VAT rate
func (ar *applicantRepository) DeactivateVATRate(tx *gorm.DB, rateID uuid.UUID, updatedBy string) (*models.VATRate, error) {
	var vatRate models.VATRate

	// First, get the current rate to return the updated version
	if err := tx.Where("id = ?", rateID).First(&vatRate).Error; err != nil {
		config.Logger.Error("Failed to find VAT rate for deactivation",
			zap.Error(err),
			zap.String("rateID", rateID.String()))
		return nil, fmt.Errorf("failed to find VAT rate: %w", err)
	}

	// Update the rate to deactivate it and set ValidTo
	now := time.Now()
	updates := map[string]interface{}{
		"is_active":  false,
		"valid_to":   now,
		"updated_by": &updatedBy,
		"updated_at": now,
	}

	if err := tx.Model(&models.VATRate{}).
		Where("id = ?", rateID).
		Updates(updates).Error; err != nil {
		config.Logger.Error("Failed to deactivate VAT rate",
			zap.Error(err),
			zap.String("rateID", rateID.String()))
		return nil, fmt.Errorf("failed to deactivate VAT rate: %w", err)
	}

	// Reload the updated rate to return it
	if err := tx.Where("id = ?", rateID).First(&vatRate).Error; err != nil {
		config.Logger.Error("Failed to reload deactivated VAT rate",
			zap.Error(err),
			zap.String("rateID", rateID.String()))
		return nil, fmt.Errorf("failed to reload VAT rate: %w", err)
	}

	return &vatRate, nil
}

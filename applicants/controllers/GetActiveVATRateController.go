package controllers

import (
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (ac *ApplicantController) GetActiveVATRateController(c *fiber.Ctx) error {
	// No need for transaction for a simple read operation
	vatRate, err := ac.ApplicantRepo.GetActiveVATRate(ac.DB)
	if err != nil {
		config.Logger.Error("Failed to fetch active VAT rate", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch active VAT rate",
			"error":   err.Error(),
		})
	}

	if vatRate == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "No active VAT rate found",
			"error":   "no_active_vat_rate",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Active VAT rate retrieved successfully",
		"data":    vatRate,
	})
}

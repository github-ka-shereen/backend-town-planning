package controllers

import (
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (elc *EnhancedLoginController) GetTrustedDeviceByUserID(c *fiber.Ctx) error {
	userID := c.Params("userID")
	if userID == "" {
		config.Logger.Error("Empty userID provided")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	devices, err := elc.deviceService.GetTrustedDevices(userID)
	if err != nil {
		config.Logger.Error("Failed to fetch devices",
			zap.String("userID", userID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to fetch devices",
			"details": err.Error(),
		})
	}

	if len(devices) == 0 {
		config.Logger.Info("No devices found for user",
			zap.String("userID", userID),
		)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "No trusted devices found for this user",
		})
	}

	// Return all devices for the user (not just first match)
	return c.JSON(fiber.Map{
		"data":  devices,
		"count": len(devices),
	})
}

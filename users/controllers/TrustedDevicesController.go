package controllers

import (
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (elc *EnhancedLoginController) GetTrustedDevices(c *fiber.Ctx) error {
	// Get user ID from context (set by auth middleware)
	userID := c.Locals("userID").(string)
	if userID == "" {
		config.Logger.Error("No user ID found in context")
		return elc.sendErrorResponse(c, fiber.StatusUnauthorized, "Authentication required", nil)
	}

	// Get trusted devices from service
	devices, err := elc.deviceService.GetTrustedDevices(userID)
	if err != nil {
		config.Logger.Error("Failed to get trusted devices",
			zap.String("userID", userID),
			zap.Error(err),
		)
		return elc.sendErrorResponse(c, fiber.StatusInternalServerError, "Failed to retrieve devices", err)
	}

	// Filter sensitive information from devices before returning
	filteredDevices := make([]map[string]interface{}, len(devices))
	for i, device := range devices {
		filteredDevices[i] = map[string]interface{}{
			"device_id":     device.DeviceID,
			"device_name":   device.DeviceName,
			"last_used_at":  device.LastUsedAt,
			"registered_at": device.RegisteredAt,
			"is_active":     device.IsActive,
			// Exclude fingerprint details for security
		}
	}

	return elc.sendSuccessResponse(c, "Trusted devices retrieved", fiber.Map{
		"devices": filteredDevices,
	})
}

func (elc *EnhancedLoginController) RemoveTrustedDevice(c *fiber.Ctx) error {
	type RemoveRequest struct {
		UserID   string `json:"userId"`
		DeviceID string `json:"deviceId"`
	}

	var req RemoveRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Failed to parse remove device request",
			zap.Error(err),
		)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Validate inputs
	if req.UserID == "" || req.DeviceID == "" {
		config.Logger.Error("Missing parameters in remove device request")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Both userID and deviceID are required",
		})
	}

	// // Optional: Verify the requesting user matches the token
	// tokenUserID := c.Locals("userID").(string)
	// if tokenUserID != req.UserID {
	// 	config.Logger.Warn("User ID mismatch in device removal",
	// 		zap.String("tokenUser", tokenUserID),
	// 		zap.String("requestUser", req.UserID),
	// 	)
	// 	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
	// 		"error": "Not authorized to remove this device",
	// 	})
	// }

	// Perform deletion
	err := elc.deviceService.RemoveTrustedDevice(req.UserID, req.DeviceID)
	if err != nil {
		config.Logger.Error("Failed to remove device",
			zap.String("userID", req.UserID),
			zap.String("deviceID", req.DeviceID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove device",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Device removed successfully",
	})
}

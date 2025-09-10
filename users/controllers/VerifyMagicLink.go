// controllers/verify_magic_link.go
package controllers

import (
	"time"
	"town-planning-backend/config"
	"town-planning-backend/users/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (elc *EnhancedLoginController) VerifyMagicLink(c *fiber.Ctx) error {
	type VerifyRequest struct {
		Token             string                     `json:"token"`
		DeviceFingerprint services.DeviceFingerprint `json:"device_fingerprint"`
		TrustDevice       bool                       `json:"trust_device,omitempty"`
	}

	var req VerifyRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Failed to parse magic link verification request",
			zap.Error(err),
		)
		return elc.sendErrorResponse(c, fiber.StatusBadRequest, "Invalid request format", err)
	}

	// Validate magic link token and device fingerprint
	magicLinkData, redirectURL, err := elc.magicLinkService.ValidateMagicLink(req.Token, req.DeviceFingerprint)
	if err != nil {
		config.Logger.Warn("Magic link validation failed",
			zap.String("token_prefix", req.Token[:8]), // Log only prefix for security
			zap.Error(err),
		)
		return elc.sendErrorResponse(c, fiber.StatusUnauthorized, "Invalid or expired magic link", err)
	}

	// Get user details
	user, err := elc.userRepo.GetUserByID(magicLinkData.UserID)
	if err != nil {
		config.Logger.Error("Failed to fetch user for magic link verification",
			zap.String("userID", magicLinkData.UserID),
			zap.Error(err),
		)
		return elc.sendErrorResponse(c, fiber.StatusNotFound, "User not found", nil)
	}

	// Generate session tokens
	accessToken, err := elc.pasetoMaker.CreateToken(user.ID.String(), 24*time.Hour)
	if err != nil {
		config.Logger.Error("Failed to generate access token",
			zap.String("userID", user.ID.String()),
			zap.Error(err),
		)
		return elc.sendErrorResponse(c, fiber.StatusInternalServerError, "Failed to create session", err)
	}

	refreshToken, err := elc.pasetoMaker.CreateToken(user.ID.String(), 7*24*time.Hour)
	if err != nil {
		config.Logger.Error("Failed to generate refresh token",
			zap.String("userID", user.ID.String()),
			zap.Error(err),
		)
		return elc.sendErrorResponse(c, fiber.StatusInternalServerError, "Failed to create session", err)
	}

	// In VerifyMagicLink controller
	if req.TrustDevice {
		isTrusted, device, err := elc.deviceService.IsDeviceTrusted(user.ID.String(), req.DeviceFingerprint)
		if err != nil {
			config.Logger.Error("Failed to check device trust status",
				zap.String("userID", user.ID.String()),
				zap.Error(err),
			)
			// Continue with login despite the error
		} else {
			if !isTrusted {
				// Device not trusted - register it
				newDevice, err := elc.deviceService.RegisterDevice(user.ID.String(), req.DeviceFingerprint)
				if err != nil {
					config.Logger.Error("Failed to register trusted device",
						zap.String("userID", user.ID.String()),
						zap.Error(err),
					)
				} else {
					config.Logger.Info("New device registered",
						zap.String("userID", user.ID.String()),
						zap.String("deviceID", newDevice.DeviceID),
					)
				}
			} else {
				// Device already trusted - just log
				config.Logger.Debug("Device already trusted",
					zap.String("userID", user.ID.String()),
					zap.String("deviceID", device.DeviceID),
				)
			}
		}
	}

	// Store refresh token in Redis
	err = elc.redisClient.Set(elc.ctx,
		"refresh_token:"+refreshToken,
		user.ID.String(),
		7*24*time.Hour,
	).Err()
	if err != nil {
		config.Logger.Error("Failed to store refresh token",
			zap.String("userID", user.ID.String()),
			zap.Error(err),
		)
		return elc.sendErrorResponse(c, fiber.StatusInternalServerError, "Failed to create session", err)
	}

	// Set cookies (same as ValidateOtp)
	accessCookie := fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Expires:  time.Now().Add(15 * time.Minute),
		HTTPOnly: true,
		Secure:   false,
		SameSite: "None",
		Path:     "/",
		Domain:   "localhost",
	}

	refreshCookie := fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   false,
		SameSite: "None",
		Path:     "/",
		Domain:   "localhost",
	}

	c.Cookie(&accessCookie)
	c.Cookie(&refreshCookie)

	// Log successful verification
	config.Logger.Info("Magic link verified successfully",
		zap.String("userID", user.ID.String()),
		zap.String("email", user.Email),
	)

	return c.JSON(fiber.Map{
		"message": "Magic link validated successfully",
		"data": fiber.Map{
			"user":         user,
			"redirect_url": redirectURL,
		},
		"error": nil,
	})
}

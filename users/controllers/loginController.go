package controllers

import (
	"context"
	"town-planning-backend/config"
	"town-planning-backend/token"
	"town-planning-backend/users/repositories"
	"town-planning-backend/users/services"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type LoginController struct {
	UserRepo    repositories.UserRepository
	PasetoMaker token.Maker
	Ctx         context.Context
	RedisClient *redis.Client
}

// Enhanced login with TOTP support
func (lc *LoginController) LoginUser(c *fiber.Ctx) error {
	type LoginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Error parsing login request body", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	user, err := lc.UserRepo.GetUserByEmail(req.Email)
	if err != nil || !services.CheckPasswordHash(req.Password, user.Password) {
		if err != nil {
			config.Logger.Warn("Login attempt: User not found or database error",
				zap.String("email", req.Email),
				zap.Error(err),
			)
		} else {
			config.Logger.Warn("Login attempt: Invalid password",
				zap.String("email", req.Email),
			)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Authentication failed",
			"data":    nil,
			"error":   "Invalid email or password.",
		})
	}

	otpService := services.NewOtpService(lc.RedisClient, lc.Ctx)

	// Check if TOTP is enabled for this user
	if otpService.IsTOTPEnabled(user.ID.String()) {
		// Generate a pre_token for the TOTP verification step
		otp, pre_token := otpService.GenerateOtp("login_otp:" + user.ID.String())

		return c.JSON(fiber.Map{
			"message": "TOTP verification required",
			"data": fiber.Map{
				"requires_totp": true,
				"user_id":       user.ID.String(),
				"pre_token":     pre_token,
				"otp":           otp,
			},
			"error": nil,
		})
	}

	// TOTP not enabled, use email OTP
	otp, pre_token := otpService.GenerateOtp("login_otp:" + user.ID.String())

	message := "Here is your OTP: " + otp
	title := "Authentication OTP"
	utils.SendEmail(user.Email, message, title, otp, "")

	return c.JSON(fiber.Map{
		"message": "OTP sent successfully",
		"data": fiber.Map{
			"requires_totp": false,
			"pre_token":     pre_token,
			"user_id":       user.ID.String(),
		},
		"error": nil,
	})
}

// Setup TOTP for a user
func (lc *LoginController) SetupTOTP(c *fiber.Ctx) error {
	type SetupRequest struct {
		UserID string `json:"user_id"`
	}

	var req SetupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	// Get user to validate they exist and get their email
	user, err := lc.UserRepo.GetUserByID(req.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
			"data":    nil,
			"error":   "User does not exist.",
		})
	}

	otpService := services.NewOtpService(lc.RedisClient, lc.Ctx)

	// Check if TOTP is already enabled
	if otpService.IsTOTPEnabled(user.ID.String()) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "TOTP already enabled",
			"data":    nil,
			"error":   "TOTP is already set up for this user.",
		})
	}

	setup, err := otpService.GenerateTOTPSecret(user.ID.String(), user.Email)
	if err != nil {
		config.Logger.Error("Failed to generate TOTP secret", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Setup failed",
			"data":    nil,
			"error":   "Failed to generate TOTP secret.",
		})
	}

	return c.JSON(fiber.Map{
		"message": "TOTP setup initiated",
		"data": fiber.Map{
			"qr_code_url":  setup.QRCodeURL,
			"manual_key":   setup.ManualKey,
			"instructions": "Scan the QR code with your authenticator app or manually enter the key. Then verify with a code to complete setup.",
		},
		"error": nil,
	})
}

// Enable TOTP after user verifies they can generate correct codes
func (lc *LoginController) EnableTOTP(c *fiber.Ctx) error {
	type EnableRequest struct {
		UserID   string `json:"user_id"`
		TOTPCode string `json:"totp_code"`
	}

	var req EnableRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	otpService := services.NewOtpService(lc.RedisClient, lc.Ctx)

	err := otpService.EnableTOTP(req.UserID, req.TOTPCode)
	if err != nil {
		config.Logger.Error("Failed to enable TOTP", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Enable failed",
			"data":    nil,
			"error":   "Invalid code or setup not found.",
		})
	}

	return c.JSON(fiber.Map{
		"message": "TOTP enabled successfully",
		"data": fiber.Map{
			"enabled": true,
		},
		"error": nil,
	})
}

// Disable TOTP for a user
func (lc *LoginController) DisableTOTP(c *fiber.Ctx) error {
	type DisableRequest struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"` // Require password for security
	}

	var req DisableRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	// Verify user's password before disabling TOTP
	user, err := lc.UserRepo.GetUserByID(req.UserID)
	if err != nil || !services.CheckPasswordHash(req.Password, user.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Authentication failed",
			"data":    nil,
			"error":   "Invalid password.",
		})
	}

	otpService := services.NewOtpService(lc.RedisClient, lc.Ctx)

	err = otpService.DisableTOTP(req.UserID)
	if err != nil {
		config.Logger.Error("Failed to disable TOTP", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Disable failed",
			"data":    nil,
			"error":   "Failed to disable TOTP.",
		})
	}

	return c.JSON(fiber.Map{
		"message": "TOTP disabled successfully",
		"data": fiber.Map{
			"enabled": false,
		},
		"error": nil,
	})
}

// Check TOTP status for a user
func (lc *LoginController) GetTOTPStatus(c *fiber.Ctx) error {
	userID := c.Params("user_id")

	otpService := services.NewOtpService(lc.RedisClient, lc.Ctx)
	enabled := otpService.IsTOTPEnabled(userID)

	return c.JSON(fiber.Map{
		"message": "TOTP status retrieved",
		"data": fiber.Map{
			"totp_enabled": enabled,
		},
		"error": nil,
	})
}

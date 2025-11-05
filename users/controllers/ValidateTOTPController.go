package controllers

import (
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/users/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (lc *LoginController) ValidateTOTP(c *fiber.Ctx) error {
	type ValidateTOTPRequest struct {
		UserId   string `json:"user_id"`
		TOTPCode string `json:"totp_code"`
		PreToken string `json:"pre_token"`
		Otp      string `json:"otp"`
	}

	var req ValidateTOTPRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Error parsing TOTP validation request body", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	otpService := services.NewOtpService(lc.RedisClient, lc.Ctx)

	// First validate the pre_token (similar to email OTP flow)
	if !otpService.ValidateOtp(req.Otp, req.PreToken, "login_otp:"+req.UserId) {
		config.Logger.Warn("Invalid pre-token provided",
			zap.String("user_id", req.UserId),
			zap.String("pre_token_hint", req.PreToken[:5]+"..."),
		)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Authentication failed",
			"data":    nil,
			"error":   "Invalid session.",
		})
	}

	// Then validate the TOTP code
	if !otpService.ValidateTOTPCode(req.UserId, req.TOTPCode) {
		config.Logger.Warn("Invalid TOTP code provided",
			zap.String("user_id", req.UserId),
		)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Authentication failed",
			"data":    nil,
			"error":   "Invalid authenticator code.",
		})
	}

	// Get user details
	user, err := lc.UserRepo.GetUserByID(req.UserId)
	if err != nil {
		config.Logger.Error("Error fetching user by ID during TOTP validation",
			zap.String("user_id", req.UserId),
			zap.Error(err),
		)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Authentication failed",
			"data":    nil,
			"error":   "Invalid user or session.",
		})
	}

	// Generate tokens (same as ValidateOtp)
	userWithoutPassword := models.User{
		ID:        user.ID,
		Active:    user.Active,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Phone:     user.Phone,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		CreatedBy: user.CreatedBy,
	}

	accessToken, err := lc.PasetoMaker.CreateToken(user.ID, 15*time.Minute)
	if err != nil {
		config.Logger.Error("Error generating access token",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Something went wrong",
			"data":    nil,
			"error":   "An internal server error occurred during token generation.",
		})
	}

	refreshToken, err := lc.PasetoMaker.CreateToken(user.ID, 7*24*time.Hour)
	if err != nil {
		config.Logger.Error("Error generating refresh token",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Something went wrong",
			"data":    nil,
			"error":   "An internal server error occurred during token generation.",
		})
	}

	err = lc.RedisClient.Set(lc.Ctx, "refresh_token:"+refreshToken, user.ID.String(), 7*24*time.Hour).Err()
	if err != nil {
		config.Logger.Error("Error storing refresh token in Redis",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Something went wrong",
			"data":    nil,
			"error":   "An internal server error occurred during session management.",
		})
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

	return c.JSON(fiber.Map{
		"message": "TOTP validated successfully",
		"data": fiber.Map{
			"user": userWithoutPassword,
		},
		"error": nil,
	})
}

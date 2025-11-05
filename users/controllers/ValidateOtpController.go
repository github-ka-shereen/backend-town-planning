package controllers

import (
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/users/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (lc *LoginController) ValidateOtp(c *fiber.Ctx) error {
	type ValidateOtpRequest struct {
		UserId   string `json:"user_id"`
		Otp      string `json:"otp"`
		PreToken string `json:"pre_token"`
	}

	var req ValidateOtpRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Error parsing OTP validation request body", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	otpService := services.NewOtpService(lc.RedisClient, lc.Ctx)
	// Pass the same distinct purpose key for validation
	if !otpService.ValidateOtp(req.Otp, req.PreToken, "login_otp:"+req.UserId) {
		config.Logger.Warn("OTP validation failed",
			zap.String("user_id", req.UserId),
			zap.String("pre_token_hint", req.PreToken[:5]+"..."),
		)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "OTP validation failed",
			"data":    nil,
			"error":   "Invalid OTP or pre-token.",
		})
	}

	user, err := lc.UserRepo.GetUserByID(req.UserId)
	if err != nil {
		config.Logger.Error("Error fetching user by ID during OTP validation",
			zap.String("user_id", req.UserId),
			zap.Error(err),
		)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Authentication failed",
			"data":    nil,
			"error":   "Invalid user or session.",
		})
	}

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
		"message": "OTP validated successfully",
		"data": fiber.Map{
			"user": userWithoutPassword,
		},
		"error": nil,
	})
}

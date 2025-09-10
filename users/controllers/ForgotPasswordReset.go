package controllers

import (
	"town-planning-backend/config"
	"town-planning-backend/users/repositories"
	"town-planning-backend/users/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (lc *EnhancedLoginController) ForgotPasswordReset(c *fiber.Ctx) error {
	type ForgotPasswordResetRequest struct {
		UserId      string `json:"user_id"`
		Otp         string `json:"otp"`
		PreToken    string `json:"pre_token"`
		NewPassword string `json:"new_password"`
	}

	var req ForgotPasswordResetRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Error parsing password reset request body", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	otpService := services.NewOtpService(lc.redisClient, lc.ctx)
	// The key in Redis is formed by "password_reset:" + UserId.
	// The pre_token from the URL is validated against the stored pre_token for that key.
	if !otpService.ValidateOtp(req.Otp, req.PreToken, "password_reset:"+req.UserId) {
		config.Logger.Warn("Password reset OTP validation failed",
			zap.String("user_id", req.UserId),
			zap.String("pre_token_hint", req.PreToken[:5]+"..."),
		)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Password reset failed",
			"data":    nil,
			"error":   "Invalid OTP or reset link (pre-token).", // Updated error message
		})
	}

	user, err := lc.userRepo.GetUserByID(req.UserId)
	if err != nil {
		config.Logger.Error("Error fetching user by ID during password reset",
			zap.String("user_id", req.UserId),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Password reset failed",
			"data":    nil,
			"error":   "Internal error: User not found.",
		})
	}

	// Validate the new password against your rules
	validationError := services.ValidatePassword(req.NewPassword)
	if validationError != "" {
		config.Logger.Warn("New password failed validation during reset",
			zap.String("user_id", req.UserId),
			zap.String("validation_error", validationError),
		)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Password reset failed",
			"data":    nil,
			"error":   validationError,
		})
	}

	hashedPassword, err := repositories.HashPassword(req.NewPassword)
	if err != nil {
		config.Logger.Error("Error hashing new password for reset", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Password reset failed",
			"data":    nil,
			"error":   "Error processing new password.",
		})
	}

	user.Password = hashedPassword
	_, err = lc.userRepo.UpdateUser(user)
	if err != nil {
		config.Logger.Error("Error updating user password in DB",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Password reset failed",
			"data":    nil,
			"error":   "Internal error: Could not update password.",
		})
	}

	// OTP/PreToken invalidated automatically by ValidateOtp if successful.
	// otpService.InvalidateOtp("password_reset:"+req.UserId) // Explicit call (optional, as ValidateOtp already does this)

	return c.JSON(fiber.Map{
		"message": "Password has been reset successfully.",
		"data":    nil,
		"error":   nil,
	})
}

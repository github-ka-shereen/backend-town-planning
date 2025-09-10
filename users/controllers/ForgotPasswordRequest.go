package controllers

import (
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/users/services" // Keep this import for the original services package
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// MODIFIED ForgotPasswordRequest method to send a link
func (lc *EnhancedLoginController) ForgotPasswordRequest(c *fiber.Ctx) error {
	type ForgotPasswordRequest struct {
		Email string `json:"email"`
	}

	var req ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Error parsing forgot password request body", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	user, err := lc.userRepo.GetUserByEmail(req.Email)
	if err != nil {
		config.Logger.Warn("Forgot password attempt: User not found", zap.String("email", req.Email), zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "User not found",
			"data":    nil,
		})
	}

	otpService := services.NewOtpService(lc.redisClient, lc.ctx)
	// Generate the OTP and pre_token (which will serve as our unique reset token)
	otp, pre_token := otpService.GenerateOtp("password_reset:" + user.ID.String())

	// --- NEW: Construct the password reset URL ---
	// You MUST configure your frontend's base URL and the specific reset path.
	// Get this from your application's config, not hardcoded in production!
	// For now, let's hardcode for demonstration.
	frontendBaseURL := "http://localhost:5173" // Umzebenzi: Replace with your actual frontend URL
	resetPath := "/reset-password"             // Replace with your actual frontend reset path

	// Construct the full reset link using the pre_token as the 'token' query parameter
	resetLink := fmt.Sprintf("%s%s?token=%s&user_id=%s", frontendBaseURL, resetPath, pre_token, user.ID.String())
	// --- END NEW ---

	// The email message now includes the clickable link
	message := fmt.Sprintf("You have requested a password reset. Please click on the link below to reset your password:\n\n%s\n\nThis link is valid for 5 minutes. If you did not request this, please ignore this email.", resetLink)
	title := "Password Reset Request"
	utils.SendEmail(user.Email, message, title, otp, "") // The last two parameters `otp` and `""` are passed but not directly used in `message`
	// You might want to update SendEmail to be more flexible, but for now, this works.

	return c.JSON(fiber.Map{
		"message": "If a matching account is found, a password reset link has been sent.", // Generic message
		"data":    nil,                                                                    // No need to send pre_token/user_id back to client, as it's in the email
		"error":   nil,
	})
}

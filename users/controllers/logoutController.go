package controllers

import (
	"time"

	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (lc *EnhancedLoginController) LogoutUser(c *fiber.Ctx) error {
	// Get refresh token
	refreshToken := c.Cookies("refresh_token")
	if refreshToken != "" {
		// Attempt to remove refresh token from Redis
		err := lc.redisClient.Del(lc.ctx, "refresh_token:"+refreshToken).Err()
		if err != nil {
			// Log the error internally if deletion fails.
			// Do NOT log the raw refreshToken directly in production logs.
			// You could log a hash of it or just that a deletion attempt for a refresh token failed.
			config.Logger.Error("Failed to delete refresh token from Redis during logout",
				zap.Error(err),
				// Consider logging a non-sensitive identifier if available, e.g.,
				// zap.String("refresh_token_prefix", refreshToken[:5]+"...")
			)
		} else {
			config.Logger.Info("Refresh token successfully deleted from Redis during logout")
		}
	} else {
		config.Logger.Debug("No refresh token found in cookies during logout attempt")
	}

	// Clear access token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour), // Expire immediately (past date)
		HTTPOnly: true,
		Secure:   false, // TODO: Set to 'true' for production when using HTTPS
		SameSite: "Lax", // TODO: Adjust 'SameSite' for production based on your frontend/backend domain setup
		Path:     "/",
		Domain:   "localhost", // TODO: Change to your actual domain for production (e.g., c.Hostname() or a config value)
	})

	// Clear refresh token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour), // Expire immediately (past date)
		HTTPOnly: true,
		Secure:   false, // TODO: Set to 'true' for production when using HTTPS
		SameSite: "Lax", // TODO: Adjust 'SameSite' for production based on your frontend/backend domain setup
		Path:     "/",
		Domain:   "localhost", // TODO: Change to your actual domain for production
	})

	config.Logger.Info("User logged out successfully",
		zap.String("client_ip", c.IP()),
	)

	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
		"data":    nil,
		"error":   nil,
	})
}

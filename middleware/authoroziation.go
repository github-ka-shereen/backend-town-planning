package middleware

import (
	"time"

	"town-planning-backend/config" // Import your config package to access config.Logger

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap" // Import zap for structured logging fields
)

// ProtectedRoute now expects an *AppContext
func ProtectedRoute(ctx *AppContext) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Retrieve tokens from cookies
		accessToken := c.Cookies("access_token")
		refreshToken := c.Cookies("refresh_token")

		// If access token exists, verify it
		if accessToken != "" {
			payload, err := ctx.PasetoMaker.VerifyToken(accessToken)
			if err == nil {
				// Valid access token, proceed
				c.Locals("user", payload)
				return c.Next()
			}
			// Log invalid access token internally, but don't expose details to client
			config.Logger.Debug("Invalid access token encountered", zap.Error(err))
		}

		// At this point, either access token is missing or invalid
		// Try to use refresh token to get a new access token
		if refreshToken == "" {
			config.Logger.Debug("No refresh token provided in request")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
				"error":   "Authentication required", // Generic error for client
			})
		}

		// Verify the refresh token
		refreshPayload, err := ctx.PasetoMaker.VerifyToken(refreshToken)
		if err != nil {
			config.Logger.Error("Invalid refresh token verification failed", zap.Error(err))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
				"error":   "Session expired or invalid. Please log in again.", // Generic error for client
			})
		}

		// Check refresh token in Redis
		// We use the full refresh token string as the key for direct lookup and invalidation.
		// TODO: Consider using a hash of the refresh token or the payload ID as the Redis key
		// if the raw token string is excessively long, for better Redis key management.
		userID, err := ctx.RedisClient.Get(ctx.Ctx, "refresh_token:"+refreshToken).Result()
		if err == redis.Nil {
			// If the refresh token is not found in Redis, it means it's either expired,
			// already used (if single-use is enforced), or invalid.
			config.Logger.Warn("Refresh token not found in Redis",
				zap.String("payload_id", refreshPayload.ID.String()),
				zap.String("email", refreshPayload.Email),
			)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized",
				"error":   "Session invalid. Please log in again.", // Generic error for client
			})
		} else if err != nil {
			// Handle other Redis errors (e.g., connection issues)
			config.Logger.Error("Error accessing Redis for refresh token validation",
				zap.String("payload_id", refreshPayload.ID.String()),
				zap.String("email", refreshPayload.Email),
				zap.Error(err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Something went wrong",
				"error":   "An internal server error occurred.", // Generic error for client
			})
		}

		// --- Single-Use Refresh Token Implementation ---

		// 1. Invalidate the old refresh token from Redis immediately after successful lookup
		err = ctx.RedisClient.Del(ctx.Ctx, "refresh_token:"+refreshToken).Err()
		if err != nil {
			// Log this error, but don't prevent token issuance.
			// This might happen if the token expired between Get and Del, or a race condition.
			config.Logger.Warn("Error deleting old refresh token from Redis",
				zap.String("user_id", userID), // Log the user ID associated with the token
				zap.Error(err),
			)
		}

		// 2. Generate a new access token
		newAccessToken, err := ctx.PasetoMaker.CreateToken(refreshPayload.Email, 15*time.Minute)
		if err != nil {
			config.Logger.Error("Could not generate new access token",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Something went wrong",
				"error":   "An internal server error occurred.", // Generic error for client
			})
		}

		// 3. Generate a new refresh token
		newRefreshToken, err := ctx.PasetoMaker.CreateToken(refreshPayload.Email, 7*24*time.Hour) // 7 days duration
		if err != nil {
			config.Logger.Error("Could not generate new refresh token",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Something went wrong",
				"error":   "An internal server error occurred.", // Generic error for client
			})
		}

		// 4. Store the new refresh token in Redis, associated with the user ID
		// The key is "refresh_token:<new_refresh_token_string>", value is the userID.
		err = ctx.RedisClient.Set(ctx.Ctx, "refresh_token:"+newRefreshToken, userID, 7*24*time.Hour).Err()
		if err != nil {
			config.Logger.Error("Error storing new refresh token in Redis",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Something went wrong",
				"error":   "An internal server error occurred.", // Generic error for client
			})
		}

		// 5. Set new access token cookie
		accessCookie := fiber.Cookie{
			Name:     "access_token",
			Value:    newAccessToken,
			Expires:  time.Now().Add(15 * time.Minute),
			HTTPOnly: true,
			Secure:   false, // TODO: Set to 'true' for production when using HTTPS
			SameSite: "Lax", // TODO: Adjust 'SameSite' for production based on your frontend/backend domain setup (e.g., "None" with Secure:true for cross-origin)
			Path:     "/",
			Domain:   "localhost", // TODO: Change to your actual domain for production (e.g., c.Hostname() or a config value)
		}
		c.Cookie(&accessCookie)

		// 6. Set new refresh token cookie
		refreshCookie := fiber.Cookie{
			Name:     "refresh_token",
			Value:    newRefreshToken,
			Expires:  time.Now().Add(7 * 24 * time.Hour), // Match Redis expiration
			HTTPOnly: true,
			Secure:   false, // TODO: Set to 'true' for production when using HTTPS
			SameSite: "Lax", // TODO: Adjust 'SameSite' for production based on your frontend/backend domain setup (e.g., "None" with Secure:true for cross-origin)
			Path:     "/",
			Domain:   "localhost", // TODO: Change to your actual domain for production
		}
		c.Cookie(&refreshCookie)

		// Set user info and continue
		c.Locals("user", refreshPayload)
		return c.Next()
	}
}

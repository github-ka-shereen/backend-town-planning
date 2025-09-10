package controllers

import (
	"context"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/users/repositories" // To fetch user details

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// AdminController handles administrative tasks related to user sessions.
type AdminController struct {
	UserRepo    repositories.UserRepository
	Ctx         context.Context
	RedisClient *redis.Client
}

// NewAdminController creates a new instance of AdminController.
func NewAdminController(userRepo repositories.UserRepository, ctx context.Context, redisClient *redis.Client) *AdminController {
	return &AdminController{
		UserRepo:    userRepo,
		Ctx:         ctx,
		RedisClient: redisClient,
	}
}

// ListActiveSessions lists all currently active user sessions by scanning Redis.
func (ac *AdminController) ListActiveSessions(c *fiber.Ctx) error {
	// Use a context with a timeout for Redis operations
	redisCtx, cancel := context.WithTimeout(ac.Ctx, 10*time.Second)
	defer cancel()

	var cursor uint64
	userSessions := make(map[string][]string)  // userID -> []refreshToken
	uniqueUserIDs := make(map[string]struct{}) // To store unique user IDs

	// Scan Redis for all refresh_token keys
	// This can be slow if you have millions of keys. Consider Redisearch or a different key structure for very large scale.
	for {
		// Keys returns a small batch of keys at a time
		keys, nextCursor, err := ac.RedisClient.Scan(redisCtx, cursor, "refresh_token:*", 100).Result()
		if err != nil {
			config.Logger.Error("Failed to scan Redis keys for active sessions", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Something went wrong",
				"error":   "Could not retrieve active sessions.",
			})
		}

		for _, key := range keys {
			// The value stored is the userID
			userID, err := ac.RedisClient.Get(redisCtx, key).Result()
			if err != nil {
				config.Logger.Warn("Failed to get userID for refresh token key",
					zap.String("redis_key", key),
					zap.Error(err),
				)
				continue // Skip this key, try others
			}
			userSessions[userID] = append(userSessions[userID], key) // Store the full key (which contains the token)
			uniqueUserIDs[userID] = struct{}{}
		}

		cursor = nextCursor
		if cursor == 0 {
			break // No more keys to scan
		}
	}

	// Fetch user details for each unique userID
	var usersWithSessions []fiber.Map
	for userID := range uniqueUserIDs {
		user, err := ac.UserRepo.GetUserByID(userID)
		if err != nil {
			config.Logger.Warn("Failed to fetch user details for active session",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			// Decide if you want to skip or include partial info. For now, we skip.
			continue
		}

		usersWithSessions = append(usersWithSessions, fiber.Map{
			"id":             user.ID.String(),
			"email":          user.Email,
			"firstName":      user.FirstName,
			"lastName":       user.LastName,
			"role":           user.Role,
			"activeSessions": len(userSessions[user.ID.String()]), // Count of sessions
			// We are not returning the actual refresh tokens to the admin frontend for security.
			// If you need to revoke a specific one, the admin frontend would need a mechanism
			// to trigger a DELETE with the token.
		})
	}

	config.Logger.Info("Admin listed active sessions", zap.Int("total_users_with_sessions", len(usersWithSessions)))
	return c.JSON(fiber.Map{
		"message": "Active sessions retrieved successfully",
		"data":    usersWithSessions,
		"error":   nil,
	})
}

// RevokeSpecificSession revokes a single refresh token session.
// Expects the full refresh token string in the URL parameter.
// DELETE /admin/sessions/:refreshToken
func (ac *AdminController) RevokeSpecificSession(c *fiber.Ctx) error {
	refreshToken := c.Params("refreshToken")
	if refreshToken == "" {
		config.Logger.Warn("Admin attempted to revoke specific session with empty token")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"error":   "Refresh token not provided.",
		})
	}

	redisCtx, cancel := context.WithTimeout(ac.Ctx, 5*time.Second)
	defer cancel()

	// Delete the specific refresh token from Redis
	cmd := ac.RedisClient.Del(redisCtx, "refresh_token:"+refreshToken)
	if cmd.Err() != nil {
		config.Logger.Error("Failed to delete specific refresh token from Redis",
			zap.String("refresh_token_prefix", refreshToken[:5]+"..."), // Log prefix, not full token
			zap.Error(cmd.Err()),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Something went wrong",
			"error":   "Could not revoke session.",
		})
	}

	deletedCount := cmd.Val()
	if deletedCount == 0 {
		config.Logger.Info("Attempted to revoke specific session, but token not found",
			zap.String("refresh_token_prefix", refreshToken[:5]+"..."),
		)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Session not found",
			"error":   "The specified session was not found or already revoked.",
		})
	}

	config.Logger.Info("Admin revoked specific session",
		zap.String("refresh_token_prefix", refreshToken[:5]+"..."),
	)
	return c.JSON(fiber.Map{
		"message": "Session revoked successfully",
		"data":    nil,
		"error":   nil,
	})
}

// RevokeAllUserSessions revokes all active refresh tokens for a given user ID.
// This is less efficient with the current Redis key structure, as it requires scanning.
// DELETE /admin/users/:userId/sessions/revoke-all
func (ac *AdminController) RevokeAllUserSessions(c *fiber.Ctx) error {
	userID := c.Params("userId")
	if userID == "" {
		config.Logger.Warn("Admin attempted to revoke all sessions for empty user ID")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"error":   "User ID not provided.",
		})
	}

	redisCtx, cancel := context.WithTimeout(ac.Ctx, 15*time.Second) // Longer timeout for potential scan
	defer cancel()

	var cursor uint64
	keysToDelete := []string{}
	foundSessionsForUser := false

	// Scan all refresh_token keys and check their values (user IDs)
	for {
		keys, nextCursor, err := ac.RedisClient.Scan(redisCtx, cursor, "refresh_token:*", 100).Result()
		if err != nil {
			config.Logger.Error("Failed to scan Redis keys for user sessions to revoke", zap.String("user_id", userID), zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Something went wrong",
				"error":   "Could not revoke user sessions.",
			})
		}

		for _, key := range keys {
			val, err := ac.RedisClient.Get(redisCtx, key).Result()
			if err != nil {
				config.Logger.Warn("Failed to get value for Redis key during user session revocation scan",
					zap.String("redis_key", key),
					zap.Error(err),
				)
				continue
			}
			if val == userID {
				keysToDelete = append(keysToDelete, key)
				foundSessionsForUser = true
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if !foundSessionsForUser {
		config.Logger.Info("No active sessions found for user to revoke", zap.String("user_id", userID))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "No active sessions found for this user.",
			"error":   nil,
		})
	}

	// Delete all found keys in a single MDel operation for efficiency
	if len(keysToDelete) > 0 {
		cmd := ac.RedisClient.Del(redisCtx, keysToDelete...)
		if cmd.Err() != nil {
			config.Logger.Error("Failed to multi-delete refresh tokens for user",
				zap.String("user_id", userID),
				zap.Int("tokens_to_delete", len(keysToDelete)),
				zap.Error(cmd.Err()),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Something went wrong",
				"error":   "Could not revoke all user sessions.",
			})
		}
		config.Logger.Info("Admin revoked all sessions for user",
			zap.String("user_id", userID),
			zap.Int64("deleted_count", cmd.Val()),
		)
	} else {
		config.Logger.Info("No sessions found to delete for user (after scan)", zap.String("user_id", userID))
	}

	return c.JSON(fiber.Map{
		"message": "All sessions for user revoked successfully",
		"data":    nil,
		"error":   nil,
	})
}

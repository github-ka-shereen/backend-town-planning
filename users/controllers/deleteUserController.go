package controllers

import (
	"town-planning-backend/config"
	"town-planning-backend/users/repositories"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (uc *UserController) DeleteUserController(c *fiber.Ctx) error {
	userID := c.Params("id")

	// --- Start Database Transaction ---
	tx := uc.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not start database transaction",
			"error":   tx.Error.Error(),
		})
	}

	// Defer rollback (will execute if panic occurs or if commit isn't called)
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during user deletion", zap.Any("panic", r))
			panic(r) // Re-throw panic after rollback
		}
	}()

	// --- Soft Delete in Database (Transaction-aware) ---
	txUserRepo := repositories.NewUserRepository(tx) // Use transaction-bound repo
	if err := txUserRepo.DeleteUser(userID); err != nil {
		tx.Rollback() // Explicit rollback (defer would catch it too)
		config.Logger.Error("Database deletion failed", zap.Error(err), zap.String("userID", userID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to delete user",
			"error":   err.Error(),
		})
	}

	// --- Delete from Bleve Index ---
	if uc.BleveRepo != nil {
		if err := uc.BleveRepo.DeleteUser(userID); err != nil {
			tx.Rollback() // Rollback DB if Bleve fails
			config.Logger.Error(
				"Bleve deletion failed - rolled back DB soft-delete",
				zap.Error(err),
				zap.String("userID", userID),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "User deletion failed due to search index error",
				"error":   err.Error(),
			})
		}
	} else {
		tx.Rollback() // Rollback if Bleve service is misconfigured
		config.Logger.Error(
			"IndexingService nil - rolled back DB soft-delete",
			zap.String("userID", userID),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Server configuration error: Indexing service unavailable",
			"error":   "indexing_service_not_configured",
		})
	}

	// --- Commit Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to finalize deletion",
			"error":   err.Error(),
		})
	}

	// --- Success ---
	utils.InvalidateCacheAsync("user")
	return c.JSON(fiber.Map{
		"message": "User deleted successfully",
	})
}

package controllers

import (
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/users/repositories"
	"town-planning-backend/users/services"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UpdateUserPayload struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	Active          bool   `json:"active"`
	Password        string `json:"password"`         // Old password for verification
	NewPassword     string `json:"new_password"`     // New password to set
	ConfirmPassword string `json:"confirm_password"` // Optional confirmation
}

func (uc *UserController) UpdateUserController(c *fiber.Ctx) error {
	id := c.Params("id")
	if _, err := uuid.Parse(id); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid user ID format",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	// --- Start Database Transaction ---
	tx := uc.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin transaction", zap.Error(tx.Error))
		return c.Status(500).JSON(fiber.Map{
			"message": "Internal server error",
			"error":   tx.Error.Error(),
		})
	}

	// Defer rollback (will execute if commit isn't called)
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during user update", zap.Any("panic", r))
			panic(r)
		}
	}()

	// --- Fetch Existing User (Transaction-aware) ---
	txUserRepo := repositories.NewUserRepository(tx) // Use transaction-bound repo
	existingUser, err := txUserRepo.GetUserByID(id)
	if err != nil {
		tx.Rollback()
		return c.Status(404).JSON(fiber.Map{
			"message": "User not found",
			"error":   err.Error(),
		})
	}

	// --- Parse and Validate Payload ---
	var payload UpdateUserPayload
	if err := c.BodyParser(&payload); err != nil {
		tx.Rollback()
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Field updates
	if payload.FirstName != "" {
		existingUser.FirstName = payload.FirstName
	}
	if payload.LastName != "" {
		existingUser.LastName = payload.LastName
	}
	if payload.Phone != "" {
		existingUser.Phone = payload.Phone
	}
	if payload.Email != "" && payload.Email != existingUser.Email {
		if validationError := services.ValidateUpdatedEmail(payload.Email, txUserRepo, id); validationError != "" {
			tx.Rollback()
			return c.Status(400).JSON(fiber.Map{
				"message": "Validation error",
				"error":   validationError,
			})
		}
		existingUser.Email = payload.Email
	}
	if payload.Role != "" {
		existingUser.Role = models.Role(payload.Role)
	}
	if payload.Active != existingUser.Active {
		existingUser.Active = payload.Active
	}

	// Password update logic (same as your existing checks)
	if payload.NewPassword != "" {
		if validationError := services.ValidatePassword(payload.NewPassword); validationError != "" {
			tx.Rollback()
			return c.Status(400).JSON(fiber.Map{
				"message": "Password validation failed",
				"error":   validationError,
			})
		}
		if !repositories.CheckPasswordHash(payload.Password, existingUser.Password) {
			tx.Rollback()
			return c.Status(401).JSON(fiber.Map{
				"message": "Invalid old password",
				"error":   "Incorrect credentials",
			})
		}
		hashedPassword, err := repositories.HashPassword(payload.NewPassword)
		if err != nil {
			tx.Rollback()
			return c.Status(500).JSON(fiber.Map{
				"message": "Password hashing failed",
				"error":   err.Error(),
			})
		}
		existingUser.Password = hashedPassword
	}

	// --- Update in Database ---
	updatedUser, err := txUserRepo.UpdateUser(existingUser)
	if err != nil {
		tx.Rollback()
		return c.Status(500).JSON(fiber.Map{
			"message": "Database update failed",
			"error":   err.Error(),
		})
	}

	// --- Update Bleve Index ---
	if uc.BleveRepo != nil {
		if err := uc.BleveRepo.UpdateUser(*updatedUser); err != nil {
			tx.Rollback() // Rollback DB if Bleve fails
			config.Logger.Error(
				"Bleve update failed - rolled back DB changes",
				zap.Error(err),
				zap.String("userID", id),
				zap.String("userEmail", updatedUser.Email),
			)
			return c.Status(500).JSON(fiber.Map{
				"message": "Search index update failed",
				"error":   err.Error(),
			})
		}
	} else {
		tx.Rollback()
		config.Logger.Error(
			"IndexingService nil - rolled back DB update",
			zap.String("userID", id),
			zap.String("userEmail", updatedUser.Email),
		)
		return c.Status(500).JSON(fiber.Map{
			"message": "Search service unavailable",
			"error":   "indexing_service_not_configured",
		})
	}

	// --- Commit Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{
			"message": "Failed to finalize update",
			"error":   err.Error(),
		})
	}

	// --- Success ---
	utils.InvalidateCacheAsync("user:" + id)
	utils.InvalidateCacheAsync("users")

	return c.JSON(fiber.Map{
		"message": "User updated successfully",
		"data": models.User{
			ID:        updatedUser.ID,
			FirstName: updatedUser.FirstName,
			LastName:  updatedUser.LastName,
			Email:     updatedUser.Email,
			Phone:     updatedUser.Phone,
			Role:      updatedUser.Role,
			Active:    updatedUser.Active,
			CreatedAt: updatedUser.CreatedAt,
			UpdatedAt: updatedUser.UpdatedAt,
		},
	})
}

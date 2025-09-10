package controllers

import (
	"context"
	indexing_repository "town-planning-backend/bleve/repositories"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/users/repositories"
	"town-planning-backend/users/services"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserController struct {
	UserRepo  repositories.UserRepository
	DB        *gorm.DB
	Ctx       context.Context
	BleveRepo indexing_repository.BleveRepositoryInterface
}

type UserBleveDocument struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

func (uc *UserController) CreateUser(c *fiber.Ctx) error {
	var user models.User

	// Hash the password before saving
	hashedPassword, err := services.HashPassword(user.Password)
	if err != nil {
		return err
	}

	// Create a new user struct for DB operations
	dbUser := user
	dbUser.Password = hashedPassword // Store the hashed password in the Password field

	if err := c.BodyParser(&user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	// --- Input Validation ---
	if validationError := services.ValidateUser(&user); validationError != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation error: " + validationError,
			"data":    nil,
			"error":   validationError,
		})
	}

	if validationError := services.ValidatePassword(user.Password); validationError != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation error: " + validationError,
			"data":    nil,
			"error":   validationError,
		})
	}

	// Validate email against existing users in the database
	if validationError := services.ValidateEmail(user.Email, uc.UserRepo); validationError != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation error: " + validationError,
			"data":    nil,
			"error":   validationError,
		})
	}

	// --- Start Database Transaction ---
	tx := uc.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not start database transaction",
			"data":    nil,
			"error":   tx.Error.Error(),
		})
	}

	// IMPORTANT: Defer rollback. This will be executed if the function returns early
	// due to an error, or if an explicit commit hasn't happened.
	defer func() {
		if r := recover(); r != nil { // Catch panics
			tx.Rollback()
			config.Logger.Error("Panic detected, rolling back transaction", zap.Any("panic_reason", r))
			panic(r) // Re-throw the panic
		}
		if tx.Error != nil {
			config.Logger.Warn("Transaction not committed, attempting rollback due to prior error", zap.Error(tx.Error))
			tx.Rollback() // Ensures rollback if any error occurred before successful commit
		}
	}()

	// Use a new repository instance tied to the transaction
	txUserRepo := repositories.NewUserRepository(tx)

	// --- Database User Creation ---
	createdUser, err := txUserRepo.CreateUser(&user)
	if err != nil {
		// The defer will handle the rollback here.
		config.Logger.Error("Failed to create user in database", zap.Error(err), zap.String("email", user.Email))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Something went wrong while creating user in the database",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	user = *createdUser // Update the local 'user' variable to match the createdUser, especially if other fields like CreatedAt were populated by GORM.

	// Invalidate cache (if applicable)
	utils.InvalidateCacheAsync("user")

	// --- Bleve Indexing  ---
	if uc.BleveRepo != nil {
		err := uc.BleveRepo.IndexSingleUser(*createdUser)
		if err != nil {
			// Explicit rollback (though defer would catch it too)
			tx.Rollback()
			config.Logger.Error(
				"CRITICAL: Failed to index user in Bleve. Rolling back database transaction.",
				zap.Error(err),
				zap.String("userID", createdUser.ID.String()),
				zap.String("userEmail", createdUser.Email),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "User could not be created because indexing failed. Please try again or contact support.",
				"data":    nil,
				"error":   err.Error(),
			})
		} else {
			config.Logger.Info("Successfully indexed user in Bleve", zap.String("userID", createdUser.ID.String()), zap.String("userEmail", createdUser.Email))
		}
	} else {
		// Explicit rollback for missing IndexingService
		tx.Rollback()
		config.Logger.Error(
			"IndexingService is nil, cannot index user. Rolling back transaction.",
			zap.String("userID", createdUser.ID.String()),
			zap.String("userEmail", createdUser.Email),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Server configuration error: Indexing service not available. Cannot create user.",
			"data":    nil,
			"error":   "indexing_service_not_configured",
		})
	}

	// --- Commit Database Transaction ---
	// If we reach here, both DB creation and Bleve indexing were successful (or Bleve was skipped due to nil service and that's an error).
	// Now, explicitly commit the transaction.
	if err := tx.Commit().Error; err != nil {
		// If commit fails, the defer will catch tx.Error and try to rollback,
		// but usually a commit failure is severe (e.g., connection lost).
		config.Logger.Error("Failed to commit database transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not commit database transaction",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	// Prepare user data to return, excluding sensitive info like password
	userWithoutPassword := models.User{
		Active:    createdUser.Active,
		FirstName: createdUser.FirstName,
		LastName:  createdUser.LastName,
		Email:     createdUser.Email,
		Phone:     createdUser.Phone,
		Role:      createdUser.Role,
		CreatedAt: createdUser.CreatedAt,
		CreatedBy: createdUser.CreatedBy,
		ID:        createdUser.ID,
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User created successfully",
		"data":    userWithoutPassword,
		"error":   nil,
	})
}

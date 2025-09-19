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
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserController struct {
	UserRepo  repositories.UserRepository
	DB        *gorm.DB
	Ctx       context.Context
	BleveRepo indexing_repository.BleveRepositoryInterface
}

// CreateUserRequest represents the request body for user creation
type CreateUserRequest struct {
	FirstName      string  `json:"first_name" validate:"required,min=2,max=100"`
	LastName       string  `json:"last_name" validate:"required,min=2,max=100"`
	Email          string  `json:"email" validate:"required,email"`
	Phone          string  `json:"phone" validate:"required,e164"`
	WhatsAppNumber *string `json:"whatsapp_number" validate:"omitempty,e164"`
	Password       string  `json:"password" validate:"required,min=8"`
	RoleID         string  `json:"role_id" validate:"required,uuid4"`
	DepartmentID   *string `json:"department_id" validate:"omitempty,uuid4"`
	Active         bool    `json:"active"`
	IsSuspended    bool    `json:"is_suspended"`
	AuthMethod     string  `json:"auth_method" validate:"oneof=magic_link password authenticator"`
	CreatedBy      string  `json:"created_by" validate:"required"`
}

func (uc *UserController) CreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	// Validate password strength
	if validationError := services.ValidatePassword(req.Password); validationError != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation error: " + validationError,
			"data":    nil,
			"error":   validationError,
		})
	}

	// Validate email against existing users
	if validationError := services.ValidateEmail(req.Email, uc.UserRepo); validationError != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation error: " + validationError,
			"data":    nil,
			"error":   validationError,
		})
	}

	// Hash the password
	hashedPassword, err := services.HashPassword(req.Password)
	if err != nil {
		config.Logger.Error("Failed to hash password", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error",
			"data":    nil,
			"error":   "password_hashing_failed",
		})
	}

	// Parse UUIDs
	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid role ID format",
			"data":    nil,
			"error":   "invalid_role_id",
		})
	}

	var departmentID *uuid.UUID
	if req.DepartmentID != nil {
		deptID, err := uuid.Parse(*req.DepartmentID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid department ID format",
				"data":    nil,
				"error":   "invalid_department_id",
			})
		}
		departmentID = &deptID
	}

	// Create user model
	user := models.User{
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Email:          req.Email,
		Phone:          req.Phone,
		WhatsAppNumber: req.WhatsAppNumber,
		Password:       hashedPassword,
		RoleID:         roleID,
		DepartmentID:   departmentID,
		Active:         req.Active,
		IsSuspended:    req.IsSuspended,
		AuthMethod:     models.AuthMethod(req.AuthMethod),
		CreatedBy:      req.CreatedBy,
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

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic detected, rolling back transaction", zap.Any("panic_reason", r))
			panic(r)
		}
	}()

	// Use transaction-bound repository
	txUserRepo := repositories.NewUserRepository(tx)

	// Create user in database
	createdUser, err := txUserRepo.CreateUser(&user)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to create user in database", zap.Error(err), zap.String("email", user.Email))

		if err.Error() == "a user with that email already exists" {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"message": "A user with this email already exists",
				"data":    nil,
				"error":   "email_already_exists",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to create user",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	// --- Bleve Indexing ---
	if uc.BleveRepo != nil {
		err := uc.BleveRepo.IndexSingleUser(*createdUser)
		if err != nil {
			tx.Rollback()
			config.Logger.Error(
				"Failed to index user in Bleve. Rolling back transaction.",
				zap.Error(err),
				zap.String("userID", createdUser.ID.String()),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "User creation failed due to indexing error",
				"data":    nil,
				"error":   "indexing_failed",
			})
		}
	} else {
		tx.Rollback()
		config.Logger.Error(
			"IndexingService is nil, cannot index user",
			zap.String("userID", createdUser.ID.String()),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Server configuration error",
			"data":    nil,
			"error":   "indexing_service_not_configured",
		})
	}

	// --- Commit Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to finalize user creation",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	// Invalidate cache
	utils.InvalidateCacheAsync("user")

	// Prepare response without sensitive data
	responseUser := map[string]interface{}{
		"id":           createdUser.ID,
		"first_name":   createdUser.FirstName,
		"last_name":    createdUser.LastName,
		"email":        createdUser.Email,
		"phone":        createdUser.Phone,
		"active":       createdUser.Active,
		"is_suspended": createdUser.IsSuspended,
		"auth_method":  createdUser.AuthMethod,
		"role_id":      createdUser.RoleID,
		"created_by":   createdUser.CreatedBy,
		"created_at":   createdUser.CreatedAt,
	}

	if createdUser.DepartmentID != nil {
		responseUser["department_id"] = createdUser.DepartmentID
	}
	if createdUser.WhatsAppNumber != nil {
		responseUser["whatsapp_number"] = createdUser.WhatsAppNumber
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User created successfully",
		"data":    responseUser,
		"error":   nil,
	})
}

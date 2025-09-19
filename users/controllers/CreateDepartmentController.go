package controllers

import (
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/users/repositories"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateDepartmentRequest represents the request body for creating a department
type CreateDepartmentRequest struct {
	Name           string  `json:"name" validate:"required,min=2,max=100"`
	Description    *string `json:"description" validate:"max=500"`
	IsActive       bool    `json:"is_active"`
	Email          *string `json:"email" validate:"omitempty,email"`
	PhoneNumber    *string `json:"phone_number" validate:"omitempty,e164"`
	OfficeLocation *string `json:"office_location" validate:"omitempty,max=200"`
	CreatedBy      string  `json:"created_by" validate:"required"`
}

func (uc *UserController) CreateDepartmentController(c *fiber.Ctx) error {

	var req CreateDepartmentRequest

	// Parse and validate request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// --- Start Database Transaction ---
	tx := uc.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not start database transaction",
			"error":   tx.Error.Error(),
		})
	}

	// Defer rollback
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during department creation", zap.Any("panic", r))
			panic(r)
		}
	}()

	// Create the department
	department := models.Department{
		ID:             uuid.New(),
		Name:           req.Name,
		Description:    req.Description,
		IsActive:       req.IsActive,
		Email:          req.Email,
		PhoneNumber:    req.PhoneNumber,
		OfficeLocation: req.OfficeLocation,
		CreatedBy:      req.CreatedBy,
	}

	txUserRepo := repositories.NewUserRepository(tx)

	createDepartment, err := txUserRepo.CreateDepartment(&department)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to create department", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not create department",
			"error":   err.Error(),
		})
	}

	// --- Commit Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to finalize department creation",
			"error":   err.Error(),
		})
	}

	// --- Success ---
	config.Logger.Info("Department created successfully",
		zap.String("departmentID", department.ID.String()),
		zap.String("departmentName", department.Name))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Department created successfully",
		"department": fiber.Map{
			"data": createDepartment,
		},
	})
}

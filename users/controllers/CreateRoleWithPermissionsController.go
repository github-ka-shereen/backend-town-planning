package controllers

import (
	"errors"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateRoleRequest represents the request body for creating a role with permissions
type CreateRoleRequest struct {
	Name        string   `json:"name" validate:"required,min=2,max=100"`
	Description string   `json:"description" validate:"max=500"`
	IsSystem    bool     `json:"is_system"`
	IsActive    bool     `json:"is_active"`
	Permissions []string `json:"permissions" validate:"required,min=1"` // Array of permission IDs
	CreatedBy   string   `json:"created_by"`
}

// CreateRoleWithPermissions creates a new role with associated permissions
func (uc *UserController) CreateRoleWithPermissionsController(c *fiber.Ctx) error {
	var req CreateRoleRequest

	// Parse and validate request
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Failed to parse request body", zap.Error(err))
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

	// Defer rollback (will execute if panic occurs or if commit isn't called)
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during role creation", zap.Any("panic", r))
			panic(r) // Re-throw panic after rollback
		}
	}()

	// Check if role with same name already exists
	var existingRole models.Role
	if err := tx.Where("name = ?", req.Name).First(&existingRole).Error; err == nil {
		tx.Rollback()
		config.Logger.Warn("Role with same name already exists", zap.String("name", req.Name))
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "Role with this name already exists",
			"error":   "role_already_exists",
		})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		tx.Rollback()
		config.Logger.Error("Database error checking existing role", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to check existing role",
			"error":   err.Error(),
		})
	}

	// Create the role
	role := models.Role{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    req.IsSystem,
		IsActive:    req.IsActive,
		CreatedBy:   req.CreatedBy,
	}

	// Create role in database
	if err := tx.Create(&role).Error; err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to create role", zap.Error(err), zap.String("roleName", req.Name))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to create role",
			"error":   err.Error(),
		})
	}

	// Validate and associate permissions
	if len(req.Permissions) > 0 {
		// Convert permission IDs to UUIDs
		permissionUUIDs := make([]uuid.UUID, 0, len(req.Permissions))
		for _, permID := range req.Permissions {
			permUUID, err := uuid.Parse(permID)
			if err != nil {
				tx.Rollback()
				config.Logger.Error("Invalid permission ID format", zap.Error(err), zap.String("permissionID", permID))
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"message": "Invalid permission ID format",
					"error":   "invalid_permission_id",
				})
			}
			permissionUUIDs = append(permissionUUIDs, permUUID)
		}

		// Check if all permissions exist and are active
		var validPermissions []models.Permission
		if err := tx.Where("id IN ? AND is_active = ?", permissionUUIDs, true).Find(&validPermissions).Error; err != nil {
			tx.Rollback()
			config.Logger.Error("Failed to validate permissions", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to validate permissions",
				"error":   err.Error(),
			})
		}

		if len(validPermissions) != len(permissionUUIDs) {
			tx.Rollback()
			config.Logger.Warn("Some permissions are invalid or inactive",
				zap.Int("requested", len(permissionUUIDs)),
				zap.Int("valid", len(validPermissions)))
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Some permissions are invalid or inactive",
				"error":   "invalid_permissions",
			})
		}

		// Create role-permission associations
		rolePermissions := make([]models.RolePermission, 0, len(permissionUUIDs))
		for _, permID := range permissionUUIDs {
			rolePermission := models.RolePermission{
				ID:           uuid.New(),
				RoleID:       role.ID,
				PermissionID: permID,
			}
			rolePermissions = append(rolePermissions, rolePermission)
		}

		// Batch insert role permissions
		if err := tx.Create(&rolePermissions).Error; err != nil {
			tx.Rollback()
			config.Logger.Error("Failed to create role permissions", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to assign permissions to role",
				"error":   err.Error(),
			})
		}
	}

	// --- Commit Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to finalize role creation",
			"error":   err.Error(),
		})
	}

	// --- Success ---
	config.Logger.Info("Role created successfully",
		zap.String("roleID", role.ID.String()),
		zap.String("roleName", role.Name),
		zap.Int("permissionsCount", len(req.Permissions)))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Role created successfully",
		"role": fiber.Map{
			"id":          role.ID,
			"name":        role.Name,
			"description": role.Description,
			"is_system":   role.IsSystem,
			"is_active":   role.IsActive,
			"permissions": req.Permissions,
			"created_by":  role.CreatedBy,
			"created_at":  role.CreatedAt,
		},
	})
}

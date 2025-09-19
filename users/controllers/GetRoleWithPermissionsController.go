package controllers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GetRoleWithPermissions gets a role with its permissions
func (uc *UserController) GetRoleWithPermissionsController(c *fiber.Ctx) error {
	roleID := c.Params("id")

	// Parse role ID
	roleUUID, err := uuid.Parse(roleID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid role ID format",
			"error":   "invalid_role_id",
		})
	}

	role, err := uc.UserRepo.GetRoleWithPermissionsByID(roleUUID.String())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to fetch role",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Role retrieved",
		"data":    role,
		"error":   nil,
	})
}

package controllers

import "github.com/gofiber/fiber/v2"

func (uc *UserController) GetAllRolesController(c *fiber.Ctx) error {
	allRoles, err := uc.UserRepo.GetAllRoles()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not retrieve roles",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Roles retrieved successfully",
		"data":    allRoles,
		"error":   nil,
	})
}

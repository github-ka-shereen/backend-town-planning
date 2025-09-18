package controllers

import "github.com/gofiber/fiber/v2"


func (uc *UserController) GetAllPermissionsController(c *fiber.Ctx) error {
	data, err := uc.UserRepo.GetAllPermissions()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error retrieving permissions",
			"data":    nil,
			"error":   err,
		})
	}
	

	return c.JSON(fiber.Map{
		"message": "Permissions retrieved",
		"data":    data,
		"error":   nil,
	})
}
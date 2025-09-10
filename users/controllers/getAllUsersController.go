package controllers

import (
	"github.com/gofiber/fiber/v2"
)

func (uc *UserController) GetAllUsersController(c *fiber.Ctx) error {
	users, err := uc.UserRepo.GetAllUsers()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error retrieving users",
			"data":    nil,
			"error":   err,
		})
	}
	

	return c.JSON(fiber.Map{
		"message": "Users retrieved",
		"data":    users,
		"error":   nil,
	})
}

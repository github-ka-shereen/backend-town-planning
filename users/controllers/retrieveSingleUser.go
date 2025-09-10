package controllers

import (
	"town-planning-backend/db/models"

	"github.com/gofiber/fiber/v2"
)

func (uc *UserController) RetrieveSingleUserController(c *fiber.Ctx) error {
	user, err := uc.UserRepo.GetUserByID(c.Params("id"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error retrieving user",
		})
	}

	userWithoutPassword := models.User{
		ID:        user.ID,
		Active:    user.Active,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Phone:     user.Phone,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		CreatedBy: user.CreatedBy,
	}

	return c.JSON(fiber.Map{
		"message": "User retrieved",
		"user":    userWithoutPassword,
	})
}

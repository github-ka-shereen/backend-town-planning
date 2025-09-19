package controllers

import "github.com/gofiber/fiber/v2"

func (uc *UserController) GetDepartmentsAllController(c *fiber.Ctx) error {

	allDepartments, err := uc.UserRepo.GetDepartmentsAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not retrieve departments",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Departments retrieved successfully",
		"data":    allDepartments,
		"error":   nil,
	})
}

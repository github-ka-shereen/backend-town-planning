package controllers

import (
	"github.com/gofiber/fiber/v2"
)

// handles the fetching of a single Application by ID
func (pc *ApplicationController) GetApplicationByIdController(c *fiber.Ctx) error {
	// Get the Application ID from the URL parameter
	applicationID := c.Params("id")

	// Fetch the Application from the repository using the ID
	application, err := pc.ApplicationRepo.GetApplicationById(applicationID)
	if err != nil {
		// If the Application is not found or an error occurs, return an error response
		return c.Status(404).JSON(fiber.Map{
			"message": "Application not found",
			"error":   err.Error(),
		})
	}

	// Return the Application data in the response
	return c.JSON(fiber.Map{
		"message": "Application retrieved successfully",
		"data":    application,
		"error":   nil,
	})
}

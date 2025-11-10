package controllers

import (
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
)

// handles the fetching of a single Application approval data by ID
func (pc *ApplicationController) GetApplicationApprovalDataController(c *fiber.Ctx) error {
	// Get the Application ID from the URL parameter
	applicationID := c.Params("id")

	// Get the current user ID from the context
	// Get user from context
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}
	senderUUID := payload.UserID

	// Fetch the Application from the repository using the ID
	application, err := pc.ApplicationRepo.GetEnhancedApplicationApprovalData(applicationID, senderUUID)
	if err != nil {
		// If the Application is not found or an error occurs, return an error response
		return c.Status(404).JSON(fiber.Map{
			"message": "Application approval data not found",
			"error":   err.Error(),
		})
	}

	// Return the Application data in the response
	return c.JSON(fiber.Map{
		"message": "Application approval data retrieved successfully",
		"data":    application,
		"error":   nil,
	})
}

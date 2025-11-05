package controllers

import (
	"github.com/gofiber/fiber/v2"
)

// GetPlanByIDController handles the fetching of a single plan by ID
func (dc *DocumentController) GetDocumentsByPlanID(c *fiber.Ctx) error {
	// Get the plan UUID from the URL parameter
	planUUID := c.Params("id") // The plan UUID is passed as a URL parameter

	// Fetch the plan from the repository using the UUID
	documents, err := dc.DocumentRepo.GetDocumentsByPlanID(planUUID)
	if err != nil {
		// If the plan is not found or an error occurs, return an error response
		return c.Status(404).JSON(fiber.Map{
			"message": "Documents not found",
			"error":   err.Error(),
		})
	}

	// Return the plan data in the response
	return c.JSON(fiber.Map{
		"message": "Documents retrieved successfully",
		"data":    documents,
		"error":   nil,
	})
}

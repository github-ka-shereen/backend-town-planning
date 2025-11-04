package controllers

import (
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RaiseIssue handles raising an issue for an application
func (ac *ApplicationController) RaiseIssueController(c *fiber.Ctx) error {
	var request RaiseIssueRequest

	// Parse incoming JSON payload
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request payload",
			"error":   err.Error(),
		})
	}

	// Validate required fields
	if request.ApplicationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Application ID is required",
		})
	}

	if request.Title == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Issue title is required",
		})
	}

	if request.Description == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Issue description is required",
		})
	}

	// Validate assignment type
	assignmentType := models.IssueAssignmentType(request.AssignmentType)

	switch assignmentType {
	case models.IssueAssignment_COLLABORATIVE,
		models.IssueAssignment_GROUP_MEMBER,
		models.IssueAssignment_SPECIFIC_USER:
		// valid
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid assignment type",
		})
	}

	// Get user from context
	userID, ok := c.Locals("userID").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid user ID format",
		})
	}

	// --- Start Database Transaction ---
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction for raising issue",
			zap.Error(tx.Error),
			zap.String("applicationID", request.ApplicationID),
			zap.String("userID", userID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error: Could not start database transaction",
			"error":   tx.Error.Error(),
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic detected during issue creation, rolling back transaction",
				zap.Any("panic_reason", r),
				zap.String("applicationID", request.ApplicationID),
				zap.String("userID", userID))
			panic(r)
		}
	}()

	// Process issue creation
	issue, err := ac.ApplicationRepo.RaiseApplicationIssue(
		tx,
		request.ApplicationID,
		userUUID,
		request.Title,
		request.Description,
		request.Priority,
		request.Category,
		request.AssignmentType,
		request.AssignedToUserID,
		request.AssignedToGroupMemberID,
	)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to raise application issue",
			zap.Error(err),
			zap.String("applicationID", request.ApplicationID),
			zap.String("userID", userID))

		statusCode := fiber.StatusInternalServerError
		if err.Error() == "user not authorized to raise issues for this application" {
			statusCode = fiber.StatusForbidden
		} else if err.Error() == "application not found" {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to raise issue: %s", err.Error()),
			"error":   err.Error(),
		})
	}

	// --- Commit Database Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction for issue creation",
			zap.Error(err),
			zap.String("applicationID", request.ApplicationID),
			zap.String("userID", userID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Issue raised successfully",
		zap.String("applicationID", request.ApplicationID),
		zap.String("userID", userID),
		zap.String("issueID", issue.ID.String()),
		zap.String("assignmentType", string(request.AssignmentType)))

	response := fiber.Map{
		"success": true,
		"message": "Issue raised successfully",
		"data": fiber.Map{
			"issue": issue,
		},
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

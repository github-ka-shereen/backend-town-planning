package controllers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"town-planning-backend/config"
	"town-planning-backend/token"
)

// RaiseIssue handles raising an issue for an application with chat thread creation
func (ac *ApplicationController) RaiseIssueController(c *fiber.Ctx) error {
	var request RaiseIssueRequest
	applicationID := c.Params("id")

	// Parse incoming JSON payload
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request payload",
			"error":   err.Error(),
		})
	}

	// Validate required fields
	if applicationID == "" {
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

	// Get user from context
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	userUUID := payload.UserID

	// --- Start Database Transaction ---
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction for raising issue",
			zap.Error(tx.Error),
			zap.String("applicationID", applicationID),
			zap.String("userID", userUUID.String()))
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
				zap.String("applicationID", applicationID),
				zap.String("userID", userUUID.String()))
			panic(r)
		}
	}()

	// Process issue creation with chat thread
	issue, chatThread, err := ac.ApplicationRepo.RaiseApplicationIssueWithChat(
		tx,
		applicationID,
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
			zap.String("applicationID", applicationID),
			zap.String("userID", userUUID.String()))

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
			zap.String("applicationID", applicationID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Issue raised successfully with chat thread",
		zap.String("applicationID", applicationID),
		zap.String("userID", userUUID.String()),
		zap.String("issueID", issue.ID.String()),
		zap.String("chatThreadID", chatThread.ID.String()),
		zap.String("assignmentType", string(request.AssignmentType)))

	response := fiber.Map{
		"success": true,
		"message": "Issue raised successfully",
		"data": fiber.Map{
			"issue":      issue,
			"chatThread": chatThread,
		},
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

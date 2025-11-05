package controllers

import (
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// RejectApplication handles application rejection by a group member
func (ac *ApplicationController) RejectApplicationController(c *fiber.Ctx) error {
	var request RejectApplicationRequest
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

	if request.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Rejection reason is required",
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
		config.Logger.Error("Failed to begin database transaction for rejection",
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
			config.Logger.Error("Panic detected during rejection, rolling back transaction",
				zap.Any("panic_reason", r),
				zap.String("applicationID", applicationID),
				zap.String("userID", userUUID.String()))
			panic(r)
		}
	}()

	// Process the rejection
	rejectionResult, err := ac.ApplicationRepo.ProcessApplicationRejection(
		tx,
		applicationID,
		userUUID,
		request.Reason,
		request.Comment,
		request.CommentType,
	)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to process application rejection",
			zap.Error(err),
			zap.String("applicationID", applicationID),
			zap.String("userID", userUUID.String()))

		statusCode := fiber.StatusInternalServerError
		if err.Error() == "user not authorized to reject this application" {
			statusCode = fiber.StatusForbidden
		} else if err.Error() == "application not found" {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reject application: %s", err.Error()),
			"error":   err.Error(),
		})
	}

	// --- Commit Database Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction for rejection",
			zap.Error(err),
			zap.String("applicationID", applicationID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Application rejected successfully",
		zap.String("applicationID", applicationID),
		zap.String("userID", userUUID.String()),
		zap.Bool("isFinalApprover", rejectionResult.IsFinalApprover))

	response := fiber.Map{
		"success": true,
		"message": "Application rejected successfully",
		"data": fiber.Map{
			"rejection_result":  rejectionResult,
			"is_final_approver": rejectionResult.IsFinalApprover,
			"current_status":    rejectionResult.ApplicationStatus,
		},
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

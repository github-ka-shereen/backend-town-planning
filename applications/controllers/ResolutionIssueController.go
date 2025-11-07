package controllers

import (
	"fmt"
	"strings"
	"town-planning-backend/applications/requests"
	"town-planning-backend/config"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// controllers/application_controller.go

// ResolveIssueController marks an issue as resolved
func (ac *ApplicationController) ResolveIssueController(c *fiber.Ctx) error {
	issueID := c.Params("id")

	var request requests.ResolveIssueRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
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

	user, err := ac.UserRepo.GetUserByID(userUUID.String())
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction for resolving issue",
			zap.Error(tx.Error),
			zap.String("issueID", issueID),
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
			config.Logger.Error("Panic detected during issue resolution, rolling back transaction",
				zap.Any("panic_reason", r),
				zap.String("issueID", issueID),
				zap.String("userID", userUUID.String()))
			panic(r)
		}
	}()

	// Get issue first to check permissions
	issue, err := ac.ApplicationRepo.GetIssueByID(issueID)
	if err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Issue not found",
			"error":   err.Error(),
		})
	}

	// Check if user can resolve this issue
	if !issue.CanUserResolveIssue(userUUID) {
		tx.Rollback()
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "You are not authorized to resolve this issue",
			"details": issue.GetRequiredResolver(),
		})
	}

	// Resolve the issue
	resolvedIssue, err := ac.ApplicationRepo.MarkIssueAsResolved(
		tx,
		issueID,
		userUUID,
		request.ResolutionComment,
	)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to resolve issue",
			zap.Error(err),
			zap.String("issueID", issueID),
			zap.String("userID", userUUID.String()))

		statusCode := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "already resolved") {
			statusCode = fiber.StatusConflict
		} else if strings.Contains(err.Error(), "not found") {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to resolve issue: %s", err.Error()),
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction for issue resolution",
			zap.Error(err),
			zap.String("issueID", issueID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Issue resolved successfully",
		zap.String("issueID", issueID),
		zap.String("userID", userUUID.String()),
		zap.String("resolvedBy", user.Email))

	return c.Status(fiber.StatusOK).JSON(requests.IssueResolutionResponse{
		Success: true,
		Message: "Issue resolved successfully",
		Data: &requests.IssueResolutionData{
			Issue:        resolvedIssue,
			ChatThreadID: resolvedIssue.ChatThreadID,
		},
	})
}

// ReopenIssueController reopens a resolved issue
func (ac *ApplicationController) ReopenIssueController(c *fiber.Ctx) error {
	issueID := c.Params("id")

	var request requests.ReopenIssueRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
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

	user, err := ac.UserRepo.GetUserByID(userUUID.String())
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction for reopening issue",
			zap.Error(tx.Error),
			zap.String("issueID", issueID),
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
			config.Logger.Error("Panic detected during issue reopening, rolling back transaction",
				zap.Any("panic_reason", r),
				zap.String("issueID", issueID),
				zap.String("userID", userUUID.String()))
			panic(r)
		}
	}()

	//ToDo: TEMPORARY: Bypass authorization for testing
	config.Logger.Info("TEMPORARY BYPASS: Allowing user to reopen issue for testing",
		zap.String("userID", userUUID.String()),
		zap.String("issueID", issueID))

	// Get issue first to check permissions
	// issue, err := ac.ApplicationRepo.GetIssueByID(issueID)
	// if err != nil {
	// 	tx.Rollback()
	// 	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Issue not found",
	// 		"error":   err.Error(),
	// 	})
	// }

	// Check if user can reopen this issue (same permissions as resolving)
	// if !issue.CanUserResolveIssue(userUUID) {
	// 	tx.Rollback()
	// 	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "You are not authorized to reopen this issue",
	// 		"details": issue.GetRequiredResolver(),
	// 	})
	// }

	// Reopen the issue
	reopenedIssue, err := ac.ApplicationRepo.ReopenIssue(tx, issueID, userUUID)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to reopen issue",
			zap.Error(err),
			zap.String("issueID", issueID),
			zap.String("userID", userUUID.String()))

		statusCode := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "not resolved") {
			statusCode = fiber.StatusConflict
		} else if strings.Contains(err.Error(), "not found") {
			statusCode = fiber.StatusNotFound
		} else if strings.Contains(err.Error(), "not authorized") {
			statusCode = fiber.StatusForbidden
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reopen issue: %s", err.Error()),
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction for issue reopening",
			zap.Error(err),
			zap.String("issueID", issueID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Issue reopened successfully",
		zap.String("issueID", issueID),
		zap.String("userID", userUUID.String()),
		zap.String("reopenedBy", user.Email))

	return c.Status(fiber.StatusOK).JSON(requests.IssueResolutionResponse{
		Success: true,
		Message: "Issue reopened successfully",
		Data: &requests.IssueResolutionData{
			Issue:        reopenedIssue,
			ChatThreadID: reopenedIssue.ChatThreadID,
		},
	})
}

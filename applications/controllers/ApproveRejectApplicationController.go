// controllers/application_approval_controller.go
package controllers

import (
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ApproveApplicationRequest struct {
	Comment       *string            `json:"comment"`
	CommentType   models.CommentType `json:"comment_type"`
}

type RejectApplicationRequest struct {
	Reason        string             `json:"reason"`
	Comment       *string            `json:"comment"`
	CommentType   models.CommentType `json:"comment_type"`
}

type RaiseIssueRequest struct {
	Title                   string                     `json:"title"`
	Description             string                     `json:"description"`
	Priority                string                     `json:"priority"`
	Category                *string                    `json:"category"`
	AssignmentType          models.IssueAssignmentType `json:"assignment_type"`
	AssignedToUserID        *uuid.UUID                 `json:"assigned_to_user_id"`
	AssignedToGroupMemberID *uuid.UUID                 `json:"assigned_to_group_member_id"`
}

type ResolveIssueRequest struct {
	IssueID    string  `json:"issue_id"`
	Resolution string  `json:"resolution"`
	Comment    *string `json:"comment"`
}

// ApproveApplication handles application approval by a group member
func (ac *ApplicationController) ApproveRejectApplicationController(c *fiber.Ctx) error {
	var request ApproveApplicationRequest
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

	// Get user from context (set by authentication middleware)
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
		config.Logger.Error("Failed to begin database transaction for approval",
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
			config.Logger.Error("Panic detected during approval, rolling back transaction",
				zap.Any("panic_reason", r),
				zap.String("applicationID", applicationID),
				zap.String("userID", userUUID.String()))
			panic(r)
		}
	}()

	// Process the approval
	approvalResult, err := ac.ApplicationRepo.ProcessApplicationApproval(
		tx,
		applicationID,
		userUUID,
		request.Comment,
		request.CommentType,
	)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to process application approval",
			zap.Error(err),
			zap.String("applicationID", applicationID),
			zap.String("userID", userUUID.String()))

		statusCode := fiber.StatusInternalServerError
		if err.Error() == "user not authorized to approve this application" {
			statusCode = fiber.StatusForbidden
		} else if err.Error() == "application not found" {
			statusCode = fiber.StatusNotFound
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to approve application: %s", err.Error()),
			"error":   err.Error(),
		})
	}

	// --- Commit Database Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction for approval",
			zap.Error(err),
			zap.String("applicationID", applicationID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Application approved successfully",
		zap.String("applicationID", applicationID),
		zap.String("userID", userUUID.String()),
		zap.Bool("isFinalApprover", approvalResult.IsFinalApprover),
		zap.Bool("readyForFinalApproval", approvalResult.ReadyForFinalApproval))

	response := fiber.Map{
		"success": true,
		"message": "Application approved successfully",
		"data": fiber.Map{
			"approval_result":          approvalResult,
			"is_final_approver":        approvalResult.IsFinalApprover,
			"ready_for_final_approval": approvalResult.ReadyForFinalApproval,
			"current_status":           approvalResult.ApplicationStatus,
		},
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

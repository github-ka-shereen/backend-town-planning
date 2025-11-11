package controllers

import (
	"fmt"
	"strings"
	"time"
	applicationRepositories "town-planning-backend/applications/repositories"
	"town-planning-backend/applications/requests"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/token"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

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
			"message": "Please log out and log in again",
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

	// ==================== CREATE RESOLUTION MESSAGE ====================
	if resolvedIssue.ChatThreadID != nil {
		resolutionMessage, err := ac.createResolutionMessage(
			tx,
			resolvedIssue,
			userUUID,
			user.Email,
			*request.ResolutionComment,
		)
		if err != nil {
			tx.Rollback() // ROLLBACK THE ENTIRE OPERATION
			config.Logger.Error("Failed to create resolution message - rolling back issue resolution",
				zap.Error(err),
				zap.String("issueID", issueID),
				zap.String("threadID", resolvedIssue.ChatThreadID.String()))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create resolution notification",
				"error":   err.Error(),
			})
		}

		if err := ac.markThreadAsResolved(tx, resolvedIssue.ChatThreadID.String()); err != nil {
			tx.Rollback() // ROLLBACK THE ENTIRE OPERATION
			config.Logger.Error("Failed to mark thread as resolved - rolling back issue resolution",
				zap.Error(err),
				zap.String("threadID", resolvedIssue.ChatThreadID.String()))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to update thread status",
				"error":   err.Error(),
			})
		}

		// 3. Only broadcast if both database operations succeeded
		ac.broadcastNewMessage(resolvedIssue.ChatThreadID.String(), *resolutionMessage, userUUID)
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
			"message": "Please log out and log in again",
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

	// TODO: TEMPORARY: Bypass authorization for testing
	config.Logger.Info("TEMPORARY BYPASS: Allowing user to reopen issue for testing",
		zap.String("userID", userUUID.String()),
		zap.String("issueID", issueID))

	// Get issue first to check permissions (when ready to enable)
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

	// ==================== CREATE REOPEN MESSAGE ====================
	if reopenedIssue.ChatThreadID != nil {
		reopenMessage, err := ac.createReopenMessage(
			tx,
			reopenedIssue,
			userUUID,
			user.FirstName,
			user.LastName,
		)
		if err != nil {
			tx.Rollback() // ROLLBACK THE ENTIRE OPERATION
			config.Logger.Error("Failed to create reopen message - rolling back issue reopening",
				zap.Error(err),
				zap.String("issueID", issueID),
				zap.String("threadID", reopenedIssue.ChatThreadID.String()))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create reopen notification",
				"error":   err.Error(),
			})
		}

		// ==================== MARK THREAD AS REOPENED ====================
		if err := ac.markThreadAsReopened(tx, reopenedIssue.ChatThreadID.String()); err != nil {
			tx.Rollback() // ROLLBACK THE ENTIRE OPERATION
			config.Logger.Error("Failed to mark thread as reopened - rolling back issue reopening",
				zap.Error(err),
				zap.String("threadID", reopenedIssue.ChatThreadID.String()))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to update thread status",
				"error":   err.Error(),
			})
		}

		// Broadcast the reopen message
		ac.broadcastNewMessage(reopenedIssue.ChatThreadID.String(), *reopenMessage, userUUID)
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

// createReopenMessage creates a system message when an issue is reopened
func (ac *ApplicationController) createReopenMessage(
	tx *gorm.DB,
	issue *models.ApplicationIssue,
	reopenedByID uuid.UUID,
	firstName string,
	lastName string,
) (*applicationRepositories.EnhancedChatMessage, error) {

	if issue.ChatThreadID == nil {
		return nil, fmt.Errorf("issue has no chat thread")
	}

	user, err := ac.UserRepo.GetUserByID(reopenedByID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Create professional reopen message content
	messageContent := fmt.Sprintf("Issue reopened by %s %s", firstName, lastName)

	// Create system message
	message := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    *issue.ChatThreadID,
		SenderID:    reopenedByID,
		Content:     messageContent,
		MessageType: models.MessageTypeSystem,
		Status:      models.MessageStatusSent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save message to database
	if err := tx.Create(&message).Error; err != nil {
		return nil, fmt.Errorf("failed to create reopen message: %w", err)
	}

	// Update thread's last activity
	if err := tx.Model(&models.ChatThread{}).
		Where("id = ?", issue.ChatThreadID).
		Updates(map[string]interface{}{
			"updated_at":       time.Now(),
			"last_activity_at": time.Now(),
		}).Error; err != nil {
		config.Logger.Warn("Failed to update thread timestamps for reopening",
			zap.Error(err),
			zap.String("threadID", issue.ChatThreadID.String()))
	}

	// Increment unread counts for other participants
	if err := ac.incrementUnreadCounts(tx, issue.ChatThreadID.String(), reopenedByID); err != nil {
		config.Logger.Warn("Failed to increment unread counts for reopen message",
			zap.Error(err),
			zap.String("threadID", issue.ChatThreadID.String()))
	}

	// Convert to enhanced message for broadcasting
	enhancedMessage := &applicationRepositories.EnhancedChatMessage{
		ID:          message.ID,
		Content:     message.Content,
		MessageType: message.MessageType,
		Status:      message.Status,
		CreatedAt:   message.CreatedAt.Format(time.RFC3339),
		Sender: &applicationRepositories.UserSummary{
			ID:        message.SenderID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Department: utils.DerefString(func() *string {
				if user.Department != nil {
					return &user.Department.Name
				}
				return nil
			}()),
		},
		ParentID:    nil,
		Attachments: nil,
	}

	return enhancedMessage, nil
}

// markThreadAsReopened reactivates a chat thread when issue is reopened
func (ac *ApplicationController) markThreadAsReopened(tx *gorm.DB, threadID string) error {
	threadUUID, err := uuid.Parse(threadID)
	if err != nil {
		return fmt.Errorf("invalid thread ID: %w", err)
	}

	now := time.Now()
	if err := tx.Model(&models.ChatThread{}).
		Where("id = ?", threadUUID).
		Updates(map[string]interface{}{
			"is_resolved": false,
			"resolved_at": nil,
			"updated_at":  now,
			"is_active":   true, // Reactivate the thread
		}).Error; err != nil {
		return fmt.Errorf("failed to mark thread as reopened: %w", err)
	}

	return nil
}

// createResolutionMessage creates a system message when an issue is resolved
func (ac *ApplicationController) createResolutionMessage(
	tx *gorm.DB,
	issue *models.ApplicationIssue,
	resolvedByID uuid.UUID,
	resolvedByEmail string,
	resolutionComment string,
) (*applicationRepositories.EnhancedChatMessage, error) {

	if issue.ChatThreadID == nil {
		return nil, fmt.Errorf("issue has no chat thread")
	}

	user, err := ac.UserRepo.GetUserByID(resolvedByID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Create resolution message content
	messageContent := fmt.Sprintf("Issue resolved by %s %s", user.FirstName, user.LastName)
	if resolutionComment != "" {
		messageContent = fmt.Sprintf("Issue resolved by %s %s:\n%s", user.FirstName, user.LastName, resolutionComment)
	}

	// Create system message
	message := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    *issue.ChatThreadID,
		SenderID:    resolvedByID,
		Content:     messageContent,
		MessageType: models.MessageTypeSystem,
		Status:      models.MessageStatusSent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save message to database
	if err := tx.Create(&message).Error; err != nil {
		return nil, fmt.Errorf("failed to create resolution message: %w", err)
	}

	// Update thread's last activity
	if err := tx.Model(&models.ChatThread{}).
		Where("id = ?", issue.ChatThreadID).
		Updates(map[string]interface{}{
			"updated_at":       time.Now(),
			"last_activity_at": time.Now(),
		}).Error; err != nil {
		config.Logger.Warn("Failed to update thread timestamps for resolution",
			zap.Error(err),
			zap.String("threadID", issue.ChatThreadID.String()))
	}

	// Increment unread counts for other participants
	if err := ac.incrementUnreadCounts(tx, issue.ChatThreadID.String(), resolvedByID); err != nil {
		config.Logger.Warn("Failed to increment unread counts for resolution message",
			zap.Error(err),
			zap.String("threadID", issue.ChatThreadID.String()))
	}

	// Convert to enhanced message for broadcasting
	// Convert to enhanced format
	enhancedMessage := &applicationRepositories.EnhancedChatMessage{
		ID:          message.ID,
		Content:     message.Content,
		MessageType: message.MessageType,
		Status:      message.Status,
		CreatedAt:   message.CreatedAt.Format(time.RFC3339),
		Sender: &applicationRepositories.UserSummary{
			ID:        message.SenderID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Department: utils.DerefString(func() *string {
				if user.Department != nil {
					return &user.Department.Name
				}
				return nil
			}()),
		},
		ParentID:    nil,
		Attachments: nil,
	}

	return enhancedMessage, nil
}

// markThreadAsResolved marks a chat thread as resolved
func (ac *ApplicationController) markThreadAsResolved(tx *gorm.DB, threadID string) error {
	threadUUID, err := uuid.Parse(threadID)
	if err != nil {
		return fmt.Errorf("invalid thread ID: %w", err)
	}

	now := time.Now()
	if err := tx.Model(&models.ChatThread{}).
		Where("id = ?", threadUUID).
		Updates(map[string]interface{}{
			"is_resolved": true,
			"resolved_at": now,
			"updated_at":  now,
			"is_active":   false, // Optional: deactivate the thread
		}).Error; err != nil {
		return fmt.Errorf("failed to mark thread as resolved: %w", err)
	}

	return nil
}

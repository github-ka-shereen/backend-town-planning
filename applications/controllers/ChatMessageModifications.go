package controllers

import (
	"mime/multipart"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Add these methods to your chat_controller.go file

// StarMessageController handles starring/unstarring a message
func (ac *ApplicationController) StarMessageController(c *fiber.Ctx) error {
	messageID := c.Params("messageId")
	if messageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Message ID is required",
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
	messageUUID, err := uuid.Parse(messageID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid message ID format",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin transaction for starring message",
			zap.Error(tx.Error),
			zap.String("messageID", messageID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error",
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during message starring",
				zap.Any("panic", r),
				zap.String("messageID", messageID))
		}
	}()

	// Star/unstar the message
	isStarred, err := ac.ApplicationRepo.StarMessage(tx, messageUUID, userUUID)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to star message",
			zap.Error(err),
			zap.String("messageID", messageID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to star message",
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction for starring message",
			zap.Error(err),
			zap.String("messageID", messageID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error",
		})
	}

	action := "starred"
	if !isStarred {
		action = "unstarred"
	}

	config.Logger.Info("Message "+action+" successfully",
		zap.String("messageID", messageID),
		zap.String("userID", userUUID.String()),
		zap.Bool("isStarred", isStarred))

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Message " + action + " successfully",
		"data": fiber.Map{
			"starred": isStarred,
		},
	})
}

// ReplyToMessageController handles replying to a message
func (ac *ApplicationController) ReplyToMessageController(c *fiber.Ctx) error {
	messageID := c.Params("messageId")
	if messageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Message ID is required",
		})
	}

	// Parse multipart form for reply content and attachments
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid form data",
			"error":   err.Error(),
		})
	}

	// Extract form values
	content := getFormValue(form, "content")
	messageType := getFormValue(form, "message_type")
	if messageType == "" {
		messageType = "TEXT"
	}

	// Get uploaded files
	var files []*multipart.FileHeader
	if form.File != nil {
		files = form.File["attachments"]
	}

	// Validate input
	if content == "" && len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Reply content or attachments are required",
			"error":   "empty_reply",
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

	// Get user details
	user, err := ac.UserRepo.GetUserByID(userUUID.String())
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Please log out and log in again",
		})
	}

	parentMessageUUID, err := uuid.Parse(messageID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid message ID format",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin transaction for replying to message",
			zap.Error(tx.Error),
			zap.String("parentMessageID", messageID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error",
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during message reply",
				zap.Any("panic", r),
				zap.String("parentMessageID", messageID))
		}
	}()

	// First, get the parent message to determine the thread ID
	var parentMessage models.ChatMessage
	if err := tx.Where("id = ? AND is_deleted = ?", parentMessageUUID, false).First(&parentMessage).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Parent message not found",
		})
	}

	// Get application ID from thread if available
	var applicationID *uuid.UUID
	if parentMessage.Thread.ApplicationID != uuid.Nil {
		applicationID = &parentMessage.Thread.ApplicationID
	}

	// Create the reply message
	chatMessageType := models.ChatMessageType(messageType)
	replyMessage, err := ac.ApplicationRepo.CreateReplyMessage(
		tx,
		parentMessage.ThreadID.String(),
		parentMessageUUID,
		content,
		chatMessageType,
		userUUID,
		files,
		applicationID,
		user.Email,
	)

	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to create reply message",
			zap.Error(err),
			zap.String("parentMessageID", messageID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to send reply",
			"error":   err.Error(),
		})
	}

	// Update thread's updated_at timestamp
	if err := tx.Model(&models.ChatThread{}).
		Where("id = ?", parentMessage.ThreadID).
		Update("updated_at", time.Now()).Error; err != nil {
		config.Logger.Warn("Failed to update thread timestamp for reply",
			zap.Error(err),
			zap.String("threadID", parentMessage.ThreadID.String()))
		// Don't fail the operation for this
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction for reply message",
			zap.Error(err),
			zap.String("parentMessageID", messageID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error",
		})
	}

	config.Logger.Info("Reply message sent successfully",
		zap.String("parentMessageID", messageID),
		zap.String("replyMessageID", replyMessage.ID.String()),
		zap.String("userID", userUUID.String()),
		zap.Int("attachmentCount", len(replyMessage.Attachments)))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Reply sent successfully",
		"data":    replyMessage,
	})
}

// DeleteMessageController handles soft deleting a message
func (ac *ApplicationController) DeleteMessageController(c *fiber.Ctx) error {
	messageID := c.Params("messageId")
	if messageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Message ID is required",
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
	messageUUID, err := uuid.Parse(messageID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid message ID format",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin transaction for deleting message",
			zap.Error(tx.Error),
			zap.String("messageID", messageID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error",
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during message deletion",
				zap.Any("panic", r),
				zap.String("messageID", messageID))
		}
	}()

	// Delete the message
	err = ac.ApplicationRepo.DeleteMessage(tx, messageUUID, userUUID)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to delete message",
			zap.Error(err),
			zap.String("messageID", messageID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete message",
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction for message deletion",
			zap.Error(err),
			zap.String("messageID", messageID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error",
		})
	}

	config.Logger.Info("Message deleted successfully",
		zap.String("messageID", messageID),
		zap.String("userID", userUUID.String()))

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Message deleted successfully",
	})
}

// GetMessageStarsController gets all stars for a message
func (ac *ApplicationController) GetMessageStarsController(c *fiber.Ctx) error {
	messageID := c.Params("messageId")
	if messageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Message ID is required",
		})
	}

	messageUUID, err := uuid.Parse(messageID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid message ID format",
		})
	}

	stars, err := ac.ApplicationRepo.GetMessageStars(messageUUID)
	if err != nil {
		config.Logger.Error("Failed to get message stars",
			zap.Error(err),
			zap.String("messageID", messageID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get message stars",
			"error":   err.Error(),
		})
	}

	// Convert to response format
	starResponse := make([]fiber.Map, len(stars))
	for i, star := range stars {
		starResponse[i] = fiber.Map{
			"id": star.ID,
			"user": fiber.Map{
				"id":         star.User.ID,
				"first_name": star.User.FirstName,
				"last_name":  star.User.LastName,
				"email":      star.User.Email,
				"department": star.User.Department.Name,
			},
			"created_at": star.CreatedAt.Format(time.RFC3339),
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"stars":      starResponse,
			"star_count": len(stars),
		},
	})
}

// GetMessageThreadController gets a message and its reply thread
func (ac *ApplicationController) GetMessageThreadController(c *fiber.Ctx) error {
	messageID := c.Params("messageId")
	if messageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Message ID is required",
		})
	}

	messageUUID, err := uuid.Parse(messageID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid message ID format",
		})
	}

	// Get user from context for access validation
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	userUUID := payload.UserID

	// First verify the user has access to this message thread
	var message models.ChatMessage
	if err := ac.DB.
		Preload("Thread").
		Preload("Thread.Participants", "user_id = ? AND is_active = ?", userUUID, true).
		Where("id = ? AND is_deleted = ?", messageUUID, false).
		First(&message).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Message not found or access denied",
		})
	}

	// Get the message thread
	threadMessages, err := ac.ApplicationRepo.GetMessageThread(messageUUID)
	if err != nil {
		config.Logger.Error("Failed to get message thread",
			zap.Error(err),
			zap.String("messageID", messageID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get message thread",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"messages": threadMessages,
		},
	})
}

// IsMessageStarredByUserController checks if current user has starred a message
func (ac *ApplicationController) IsMessageStarredByUserController(c *fiber.Ctx) error {
	messageID := c.Params("messageId")
	if messageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Message ID is required",
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
	messageUUID, err := uuid.Parse(messageID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid message ID format",
		})
	}

	isStarred, err := ac.ApplicationRepo.IsMessageStarredByUser(messageUUID, userUUID)
	if err != nil {
		config.Logger.Error("Failed to check star status",
			zap.Error(err),
			zap.String("messageID", messageID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to check star status",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"starred": isStarred,
		},
	})
}

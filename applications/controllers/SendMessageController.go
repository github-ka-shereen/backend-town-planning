// controllers/chat_controller.go
package controllers

import (
	"fmt"
	"mime/multipart"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SendMessageController handles sending a new chat message with optional attachments
func (ac *ApplicationController) SendMessageController(c *fiber.Ctx) error {
	// Get thread ID from URL parameters
	threadID := c.Params("threadId")
	if threadID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Thread ID is required",
			"error":   "missing_thread_id",
		})
	}

	// Parse multipart form
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
			"message": "Message content or attachments are required",
			"error":   "empty_message",
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
			"message": "User not found",
		})
	}

	// Validate message type
	chatMessageType := models.ChatMessageType(messageType)
	// if !chatMessageType {
	// 	chatMessageType = models.MessageTypeText
	// }

	// --- Start Database Transaction ---
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction for sending message",
			zap.Error(tx.Error),
			zap.String("threadID", threadID),
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
			config.Logger.Error("Panic detected during message creation, rolling back transaction",
				zap.Any("panic_reason", r),
				zap.String("threadID", threadID),
				zap.String("userID", userUUID.String()))
			panic(r)
		}
	}()

	// Verify thread exists and user is a participant
	thread, err := ac.verifyThreadAccess(tx, threadID, userUUID)
	if err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "Access denied to thread",
			"error":   err.Error(),
		})
	}

	// Get application ID from thread if available
	var applicationID *uuid.UUID
	if thread.ApplicationID != uuid.Nil {
		applicationID = &thread.ApplicationID
	}

	// Create message with attachments
	enhancedMessage, err := ac.ApplicationRepo.CreateMessageWithAttachments(
		tx,
		c,
		threadID,
		content,
		chatMessageType,
		userUUID,
		files,
		applicationID,
		user.Email,
	)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to create chat message",
			zap.Error(err),
			zap.String("threadID", threadID),
			zap.String("userID", userUUID.String()))

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to send message",
			"error":   err.Error(),
		})
	}

	// Update thread's updated_at timestamp
	if err := tx.Model(&models.ChatThread{}).
		Where("id = ?", threadID).
		Update("updated_at", time.Now()).Error; err != nil {
		config.Logger.Warn("Failed to update thread timestamp",
			zap.Error(err),
			zap.String("threadID", threadID))
		// Don't fail the entire operation for this
	}

	// --- Commit Database Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction for message creation",
			zap.Error(err),
			zap.String("threadID", threadID),
			zap.String("userID", userUUID.String()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Message sent successfully",
		zap.String("threadID", threadID),
		zap.String("userID", userUUID.String()),
		zap.String("messageID", enhancedMessage.ID.String()),
		zap.Int("attachmentCount", len(enhancedMessage.Attachments)))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Message sent successfully",
		"data":    enhancedMessage,
	})
}

// verifyThreadAccess verifies the thread exists and user has access
func (ac *ApplicationController) verifyThreadAccess(tx *gorm.DB, threadID string, userID uuid.UUID) (*models.ChatThread, error) {
	var thread models.ChatThread

	// First, verify thread exists
	if err := tx.Where("id = ? AND is_active = ?", threadID, true).First(&thread).Error; err != nil {
		return nil, fmt.Errorf("thread not found or inactive")
	}

	// Check if user is a participant in this thread
	var participant models.ChatParticipant
	if err := tx.Where("thread_id = ? AND user_id = ? AND is_active = ?", threadID, userID, true).First(&participant).Error; err != nil {
		return nil, fmt.Errorf("user is not a participant in this thread")
	}

	return &thread, nil
}

// Helper function to get form value
func getFormValue(form *multipart.Form, key string) string {
	if values, exists := form.Value[key]; exists && len(values) > 0 {
		return values[0]
	}
	return ""
}

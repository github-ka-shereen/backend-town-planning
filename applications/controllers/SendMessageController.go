// controllers/chat_controller.go
package controllers

import (
	"fmt"
	"mime/multipart"
	"time"
	applicationRepositories "town-planning-backend/applications/repositories"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/token"
	"town-planning-backend/websocket"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SendMessageController handles sending a new chat message with optional attachments and real-time broadcasting
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

	// Update thread's updated_at and last_activity_at timestamps
	now := time.Now()
	if err := tx.Model(&models.ChatThread{}).
		Where("id = ?", threadID).
		Updates(map[string]interface{}{
			"updated_at":       now,
			"last_activity_at": now,
		}).Error; err != nil {
		config.Logger.Warn("Failed to update thread timestamps",
			zap.Error(err),
			zap.String("threadID", threadID))
		// Don't fail the entire operation for this
	}

	// Increment unread counts for all participants except sender
	if err := ac.incrementUnreadCounts(tx, threadID, userUUID); err != nil {
		config.Logger.Warn("Failed to increment unread counts",
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

	// BROADCAST MESSAGE VIA WEBSOCKET FOR REAL-TIME UPDATES
	ac.broadcastNewMessage(threadID, *enhancedMessage, userUUID)

	// Also send typing stop indicator
	ac.broadcastTypingIndicator(threadID, userUUID, false)

	config.Logger.Info("Message sent and broadcasted successfully",
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

// HandleTypingIndicator handles typing indicator requests
func (ac *ApplicationController) HandleTypingIndicator(c *fiber.Ctx) error {
	threadID := c.Params("threadId")
	var req struct {
		IsTyping bool `json:"isTyping"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
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

	ac.broadcastTypingIndicator(threadID, payload.UserID, req.IsTyping)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Typing indicator sent",
	})
}

// MarkMessagesAsRead handles read receipt requests
func (ac *ApplicationController) MarkMessagesAsRead(c *fiber.Ctx) error {
	threadID := c.Params("threadId")

	var req struct {
		MessageIDs []string `json:"messageIds"`
		ReadAt     string   `json:"readAt,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
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

	// Update read receipts in database
	readAt := time.Now()
	if req.ReadAt != "" {
		if parsedTime, err := time.Parse(time.RFC3339, req.ReadAt); err == nil {
			readAt = parsedTime
		}
	}

	// Create read receipts and update message status
	for _, msgID := range req.MessageIDs {
		messageUUID, err := uuid.Parse(msgID)
		if err != nil {
			continue
		}

		// Create read receipt
		readReceipt := models.ReadReceipt{
			ID:         uuid.New(),
			MessageID:  messageUUID,
			UserID:     payload.UserID,
			ReadAt:     readAt,
			IsRealtime: true,
		}

		if err := ac.DB.Create(&readReceipt).Error; err != nil {
			config.Logger.Warn("Failed to create read receipt",
				zap.Error(err),
				zap.String("messageID", msgID),
				zap.String("userID", payload.UserID.String()))
		}

		// Update message read count
		if err := ac.DB.Model(&models.ChatMessage{}).
			Where("id = ?", messageUUID).
			UpdateColumn("read_count", gorm.Expr("read_count + ?", 1)).Error; err != nil {
			config.Logger.Warn("Failed to update message read count",
				zap.Error(err),
				zap.String("messageID", msgID))
		}
	}

	// Reset unread count for this user in this thread
	if err := ac.DB.Model(&models.ChatParticipant{}).
		Where("thread_id = ? AND user_id = ?", threadID, payload.UserID).
		Updates(map[string]interface{}{
			"unread_count": 0,
			"last_read_at": readAt,
		}).Error; err != nil {
		config.Logger.Warn("Failed to reset unread count",
			zap.Error(err),
			zap.String("threadID", threadID),
			zap.String("userID", payload.UserID.String()))
	}

	// Broadcast read receipt via WebSocket
	ac.broadcastReadReceipt(threadID, payload.UserID, req.MessageIDs)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Messages marked as read",
	})
}

// GetUnreadCount returns unread message count for a thread
func (ac *ApplicationController) GetUnreadCount(c *fiber.Ctx) error {
	threadID := c.Params("threadId")

	// Get user from context
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	unreadCount, err := ac.ApplicationRepo.GetUnreadMessageCount(threadID, payload.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get unread count",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"unreadCount": unreadCount,
			"threadId":    threadID,
			"userId":      payload.UserID,
		},
	})
}

// ==================== REAL-TIME BROADCASTING METHODS ====================

// broadcastNewMessage broadcasts a new message to all thread participants
func (ac *ApplicationController) broadcastNewMessage(threadID string, message applicationRepositories.EnhancedChatMessage, senderID uuid.UUID) {
    if ac.WsHub == nil {
        config.Logger.Warn("WebSocket hub not initialized, skipping broadcast")
        return
    }

    // Create WebSocket message with just the message data
    wsMessage := websocket.WebSocketMessage{
        Type:      websocket.MessageTypeChat,
        Payload:   message, // This should be just the EnhancedChatMessage, not wrapped
        Timestamp: time.Now(),
        ThreadID:  threadID,
    }

    // Broadcast to all clients subscribed to this thread (excluding sender)
    ac.WsHub.BroadcastToThread(threadID, wsMessage, senderID)

    config.Logger.Debug("Message broadcasted via WebSocket",
        zap.String("threadID", threadID),
        zap.String("messageID", message.ID.String()),
        zap.String("senderID", senderID.String()),
        zap.Any("wsMessage", wsMessage)) // Add this for debugging
}

// broadcastTypingIndicator broadcasts typing status to thread participants
func (ac *ApplicationController) broadcastTypingIndicator(threadID string, userID uuid.UUID, isTyping bool) {
	if ac.WsHub == nil {
		return
	}

	// Get user details for the indicator
	user, err := ac.UserRepo.GetUserByID(userID.String())
	if err != nil {
		config.Logger.Warn("Failed to get user details for typing indicator",
			zap.Error(err),
			zap.String("userID", userID.String()))
		return
	}

	typingPayload := map[string]interface{}{
		"userId":   userID,
		"userName": user.FirstName + " " + user.LastName,
		"isTyping": isTyping,
		"threadId": threadID,
	}

	wsMessage := websocket.WebSocketMessage{
		Type:      websocket.MessageTypeTyping,
		Payload:   typingPayload,
		Timestamp: time.Now(),
		ThreadID:  threadID,
	}

	ac.WsHub.BroadcastToThread(threadID, wsMessage, userID)

	config.Logger.Debug("Typing indicator broadcasted",
		zap.String("threadID", threadID),
		zap.String("userID", userID.String()),
		zap.Bool("isTyping", isTyping))
}

// broadcastReadReceipt broadcasts read receipts to thread participants
func (ac *ApplicationController) broadcastReadReceipt(threadID string, userID uuid.UUID, messageIDs []string) {
	if ac.WsHub == nil {
		return
	}

	user, err := ac.UserRepo.GetUserByID(userID.String())
	if err != nil {
		config.Logger.Warn("Failed to get user details for read receipt",
			zap.Error(err),
			zap.String("userID", userID.String()))
		return
	}

	readPayload := map[string]interface{}{
		"userId":     userID,
		"userName":   user.FirstName + " " + user.LastName,
		"messageIds": messageIDs,
		"readAt":     time.Now().Format(time.RFC3339),
		"threadId":   threadID,
	}

	wsMessage := websocket.WebSocketMessage{
		Type:      websocket.MessageTypeReadReceipt,
		Payload:   readPayload,
		Timestamp: time.Now(),
		ThreadID:  threadID,
	}

	ac.WsHub.BroadcastToThread(threadID, wsMessage, userID)

	config.Logger.Debug("Read receipt broadcasted",
		zap.String("threadID", threadID),
		zap.String("userID", userID.String()),
		zap.Int("messageCount", len(messageIDs)))
}

// ==================== HELPER METHODS ====================

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

// incrementUnreadCounts increments unread counts for all participants except sender
func (ac *ApplicationController) incrementUnreadCounts(tx *gorm.DB, threadID string, senderID uuid.UUID) error {
	// Increment participant unread counts
	if err := tx.Model(&models.ChatParticipant{}).
		Where("thread_id = ? AND user_id != ? AND is_active = ?", threadID, senderID, true).
		UpdateColumn("unread_count", gorm.Expr("unread_count + ?", 1)).Error; err != nil {
		return fmt.Errorf("failed to increment participant unread counts: %w", err)
	}

	// Increment thread unread count
	if err := tx.Model(&models.ChatThread{}).
		Where("id = ?", threadID).
		UpdateColumn("unread_count", gorm.Expr("unread_count + ?", 1)).Error; err != nil {
		return fmt.Errorf("failed to increment thread unread count: %w", err)
	}

	return nil
}

// Helper function to get form value
func getFormValue(form *multipart.Form, key string) string {
	if values, exists := form.Value[key]; exists && len(values) > 0 {
		return values[0]
	}
	return ""
}

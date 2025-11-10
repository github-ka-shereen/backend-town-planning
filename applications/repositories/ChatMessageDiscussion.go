// repositories/chat_repository.go
package repositories

import (
	"errors"
	"fmt"
	"mime/multipart"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	documents_requests "town-planning-backend/documents/requests"
	"town-planning-backend/utils"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DocumentServiceInterface interface {
	UnifiedCreateDocument(
		tx *gorm.DB,
		c *fiber.Ctx,
		request *documents_requests.CreateDocumentRequest,
		fileBytes []byte,
		fileHeader *multipart.FileHeader,
	) (*documents_requests.CreateDocumentRequest, error)
}

// CreateMessageWithAttachments creates a message with file attachments using existing document service
func (r *applicationRepository) CreateMessageWithAttachments(
	tx *gorm.DB,
	c *fiber.Ctx,
	threadID string,
	content string,
	messageType models.ChatMessageType,
	senderID uuid.UUID,
	files []*multipart.FileHeader,
	applicationID *uuid.UUID,
	createdBy string,
) (*EnhancedChatMessage, error) {

	// Validate thread ID
	threadUUID, err := uuid.Parse(threadID)
	if err != nil {
		return nil, fmt.Errorf("invalid thread ID: %w", err)
	}

	// Create the message
	message := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    threadUUID,
		SenderID:    senderID,
		Content:     content,
		MessageType: messageType,
		Status:      models.MessageStatusSent,
		CreatedAt:   time.Now(),
	}

	if err := tx.Create(&message).Error; err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	config.Logger.Info("Chat message created successfully",
		zap.String("messageID", message.ID.String()),
		zap.String("threadID", threadID))

	// Handle file attachments
	var attachments []*ChatAttachmentSummary
	var attachmentErrors []string

	for _, fileHeader := range files {
		// Use the existing document service to create the document
		documentRequest := &documents_requests.CreateDocumentRequest{
			CategoryCode:  "CHAT_ATTACHMENT",
			FileName:      fileHeader.Filename,
			CreatedBy:     createdBy,
			ApplicationID: applicationID,
			FileType:      fileHeader.Header.Get("Content-Type"),
		}

		// Use your existing document service
		response, err := r.documentSvc.UnifiedCreateDocument(
			tx, // Use the same transaction
			c,
			documentRequest,
			nil, // No file content bytes, we'll use the multipart file
			fileHeader,
		)

		if err != nil {
			errorMsg := fmt.Sprintf("failed to create document for %s: %v", fileHeader.Filename, err)
			attachmentErrors = append(attachmentErrors, errorMsg)
			config.Logger.Error("Failed to create document for chat attachment",
				zap.Error(err),
				zap.String("filename", fileHeader.Filename))
			continue
		}

		// FIXED: Use the Document from the response directly
		if response.Document == nil {
			errorMsg := fmt.Sprintf("document response is nil for %s", fileHeader.Filename)
			attachmentErrors = append(attachmentErrors, errorMsg)
			config.Logger.Error("Document response is nil",
				zap.String("filename", fileHeader.Filename))
			continue
		}

		// Create chat attachment linking to the document
		chatAttachment := models.ChatAttachment{
			ID:         uuid.New(),
			MessageID:  message.ID,
			DocumentID: response.Document.ID,
		}

		if err := tx.Create(&chatAttachment).Error; err != nil {
			errorMsg := fmt.Sprintf("failed to create chat attachment for %s: %v", fileHeader.Filename, err)
			attachmentErrors = append(attachmentErrors, errorMsg)
			config.Logger.Error("Failed to create chat attachment",
				zap.Error(err),
				zap.String("documentID", response.Document.ID.String()),
				zap.String("filename", fileHeader.Filename))
			continue
		}

		// Convert file size to string
		fileSizeStr := response.Document.FileSize

		// For frontend response
		attachments = append(attachments, &ChatAttachmentSummary{
			ID:        chatAttachment.ID,
			FileName:  response.Document.FileName,
			FileSize:  fileSizeStr.String(),
			FileType:  string(response.Document.DocumentType),
			MimeType:  response.Document.MimeType,
			FilePath:  response.Document.FilePath,
			CreatedAt: string(response.Document.CreatedAt.Format(time.RFC3339)),
		})

		config.Logger.Info("Chat attachment created successfully",
			zap.String("filename", fileHeader.Filename),
			zap.String("documentID", response.Document.ID.String()),
			zap.String("messageID", message.ID.String()),
			zap.String("chatAttachmentID", chatAttachment.ID.String()))
	}

	// Log any attachment errors but don't fail the entire message
	if len(attachmentErrors) > 0 {
		config.Logger.Warn("Some attachments failed to process",
			zap.Strings("errors", attachmentErrors),
			zap.String("messageID", message.ID.String()),
			zap.Int("successfulAttachments", len(attachments)),
			zap.Int("failedAttachments", len(attachmentErrors)))
	}

	// Use the same transaction to load the complete message
	var completeMessage models.ChatMessage
	if err := tx.
		Preload("Sender").
		Preload("Sender.Role").
		Preload("Sender.Department").
		Preload("Attachments").
		Preload("Attachments.Document").
		Where("id = ?", message.ID).
		First(&completeMessage).Error; err != nil {
		return nil, fmt.Errorf("failed to load complete message: %w", err)
	}

	// Build attachments from preloaded data (in case some attachments were created but we couldn't build summaries)
	if len(completeMessage.Attachments) > 0 && len(attachments) == 0 {
		attachments = make([]*ChatAttachmentSummary, len(completeMessage.Attachments))
		for i, attachment := range completeMessage.Attachments {
			fileSizeStr := attachment.Document.FileSize.String()

			attachments[i] = &ChatAttachmentSummary{
				ID:        attachment.ID,
				FileName:  attachment.Document.FileName,
				FileSize:  fileSizeStr,
				FileType:  string(attachment.Document.DocumentType),
				MimeType:  attachment.Document.MimeType,
				FilePath:  attachment.Document.FilePath,
				CreatedAt: attachment.Document.CreatedAt.Format(time.RFC3339),
			}
		}
	}

	// Convert to enhanced format
	enhancedMessage := &EnhancedChatMessage{
		ID:          completeMessage.ID,
		Content:     completeMessage.Content,
		MessageType: completeMessage.MessageType,
		Status:      completeMessage.Status,
		IsEdited:    completeMessage.IsEdited,
		EditedAt:    utils.FormatTimePointer(completeMessage.EditedAt),
		IsDeleted:   completeMessage.IsDeleted,
		CreatedAt:   completeMessage.CreatedAt.Format(time.RFC3339),
		Sender: &UserSummary{
			ID:        completeMessage.Sender.ID,
			FirstName: completeMessage.Sender.FirstName,
			LastName:  completeMessage.Sender.LastName,
			Email:     completeMessage.Sender.Email,
			Department: utils.DerefString(func() *string {
				if completeMessage.Sender.Department != nil {
					return &completeMessage.Sender.Department.Name
				}
				return nil
			}()),
		},
		ParentID:    completeMessage.ParentID,
		Attachments: attachments,
	}

	return enhancedMessage, nil
}

// GetChatMessagesWithPreload gets messages with all relationships preloaded
// repositories/application_repository.go

func (r *applicationRepository) GetChatMessagesWithPreload(threadID string, limit, offset int) ([]FrontendChatMessage, int64, error) {
	var messages []models.ChatMessage

	// Get total count
	var total int64
	if err := r.db.Model(&models.ChatMessage{}).
		Where("thread_id = ? AND is_deleted = ?", threadID, false).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated messages with ALL relationships preloaded including read receipts
	if err := r.db.
		Preload("Sender").
		Preload("Sender.Role").
		Preload("Sender.Department").
		Preload("Attachments").
		Preload("Attachments.Document").
		Preload("Parent").
		Preload("Parent.Sender").
		Preload("ReadReceipts").      // NEW: Preload read receipts
		Preload("ReadReceipts.User"). // NEW: Preload users who read
		Where("thread_id = ? AND is_deleted = ?", threadID, false).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	// Get thread participants count for delivered status
	var participantCount int64
	r.db.Model(&models.ChatParticipant{}).
		Where("thread_id = ? AND is_active = ?", threadID, true).
		Count(&participantCount)

	// Convert to enhanced format with read receipt data
	enhancedMessages := make([]FrontendChatMessage, len(messages))
	for i, message := range messages {
		// Build attachments from preloaded data
		attachments := make([]*models.ChatAttachment, len(message.Attachments))
		for j := range message.Attachments {
			attachments[j] = &message.Attachments[j]
		}

		// Build read receipt data
		readBy := make([]ReadReceiptUser, 0)
		for _, rr := range message.ReadReceipts {
			if rr.UserID != uuid.Nil && rr.User.ID != uuid.Nil {
				readBy = append(readBy, ReadReceiptUser{
					ID:       rr.UserID,
					FullName: rr.User.FirstName + " " + rr.User.LastName,
					Email:    rr.User.Email,
				})
			}
		}

		enhancedMessages[i] = FrontendChatMessage{
			ID:          message.ID,
			Content:     message.Content,
			MessageType: message.MessageType,
			Status:      message.Status,
			IsEdited:    message.IsEdited,
			EditedAt:    utils.FormatTimePointer(message.EditedAt),
			IsDeleted:   message.IsDeleted,
			CreatedAt:   message.CreatedAt.Format(time.RFC3339),
			Sender:      &message.Sender,
			ParentID:    message.ParentID,
			Parent:      message.Parent,
			Attachments: attachments,
			ReadCount:   message.ReadCount,
			StarCount:   message.StarCount,
			// IsStarred:        message.IsStarred,
			ReadBy:           readBy,
			DeliveredToCount: int(participantCount) - 1, // All participants except sender
		}
	}

	return enhancedMessages, total, nil
}

// GetUnreadMessageCount returns count of unread messages for a user in a thread
func (r *applicationRepository) GetUnreadMessageCount(threadID string, userID uuid.UUID) (int, error) {
	var count int64

	err := r.db.Model(&models.ChatMessage{}).
		Joins("LEFT JOIN read_receipts ON chat_messages.id = read_receipts.message_id AND read_receipts.user_id = ?", userID).
		Where("chat_messages.thread_id = ? AND chat_messages.sender_id != ? AND chat_messages.is_deleted = ? AND read_receipts.id IS NULL",
			threadID, userID, false).
		Count(&count).Error

	return int(count), err
}

// GetChatThreadByIssueID gets a chat thread by issue ID
func (r *applicationRepository) GetChatThreadByIssueID(issueID uuid.UUID) (*models.ChatThread, error) {
	var thread models.ChatThread
	if err := r.db.
		Preload("Participants.User").
		Preload("Participants.User.Role").
		Preload("Participants.User.Department").
		Where("issue_id = ?", issueID).
		First(&thread).Error; err != nil {
		return nil, err
	}
	return &thread, nil
}

// StarMessage function uses many-to-many:
func (r *applicationRepository) StarMessage(tx *gorm.DB, messageID uuid.UUID, userID uuid.UUID) (bool, error) {
	// Check if message exists and user has access
	var message models.ChatMessage
	if err := tx.
		Preload("Thread").
		Preload("Thread.Participants", "user_id = ? AND is_active = ?", userID, true).
		Where("id = ? AND is_deleted = ?", messageID, false).
		First(&message).Error; err != nil {
		return false, fmt.Errorf("message not found or access denied: %w", err)
	}

	// Check if user is already starring this message
	count := tx.Model(&message).Where("user_id = ?", userID).Association("StarredBy").Count()
	if count > 0 {
		// Star exists, so remove it (unstar)
		if err := tx.Model(&message).Association("StarredBy").Delete("user_id = ?", userID); err != nil {
			return false, fmt.Errorf("failed to unstar message: %w", err)
		}
		return false, nil
	}

	// Star doesn't exist, so create it
	user := models.User{ID: userID}
	if err := tx.Model(&message).Association("StarredBy").Append(&user); err != nil {
		return false, fmt.Errorf("failed to star message: %w", err)
	}

	return true, nil
}

// GetMessageStars gets all stars for a message with user details
func (r *applicationRepository) GetMessageStars(messageID uuid.UUID) ([]models.MessageStar, error) {
	var stars []models.MessageStar

	err := r.db.
		Preload("User").
		Preload("User.Department").
		Where("message_id = ?", messageID).
		Order("created_at ASC").
		Find(&stars).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch message stars: %w", err)
	}

	return stars, nil
}

// IsMessageStarredByUser checks if a message is starred by a specific user
func (r *applicationRepository) IsMessageStarredByUser(messageID uuid.UUID, userID uuid.UUID) (bool, error) {
	var star models.MessageStar
	err := r.db.Where("message_id = ? AND user_id = ?", messageID, userID).First(&star).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check star status: %w", err)
	}

	return true, nil
}

// DeleteMessage soft deletes a message (marks as deleted)
func (r *applicationRepository) DeleteMessage(tx *gorm.DB, messageID uuid.UUID, userID uuid.UUID) error {
	var message models.ChatMessage

	// Check if message exists and user is the sender
	if err := tx.Where("id = ? AND sender_id = ? AND is_deleted = ?", messageID, userID, false).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("message not found or you are not authorized to delete it")
		}
		return fmt.Errorf("failed to fetch message: %w", err)
	}

	// Soft delete the message
	now := time.Now()
	message.IsDeleted = true
	message.DeletedAt = &now
	message.UpdatedAt = now

	if err := tx.Save(&message).Error; err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	config.Logger.Info("Message soft deleted successfully",
		zap.String("messageID", messageID.String()),
		zap.String("userID", userID.String()))

	return nil
}

// CreateReplyMessage creates a reply to an existing message
func (r *applicationRepository) CreateReplyMessage(
	tx *gorm.DB,
	threadID string,
	parentMessageID uuid.UUID,
	content string,
	messageType models.ChatMessageType,
	senderID uuid.UUID,
	files []*multipart.FileHeader,
	applicationID *uuid.UUID,
	createdBy string,
) (*EnhancedChatMessage, error) {

	// Validate parent message exists and belongs to the same thread
	var parentMessage models.ChatMessage
	if err := tx.Where("id = ? AND thread_id = ? AND is_deleted = ?",
		parentMessageID, threadID, false).First(&parentMessage).Error; err != nil {
		return nil, fmt.Errorf("parent message not found or invalid: %w", err)
	}

	// Create the reply message with parent reference
	message := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    parentMessage.ThreadID, // Same thread as parent
		SenderID:    senderID,
		Content:     content,
		MessageType: messageType,
		Status:      models.MessageStatusSent,
		ParentID:    &parentMessageID, // Set the parent reference
		CreatedAt:   time.Now(),
	}

	if err := tx.Create(&message).Error; err != nil {
		return nil, fmt.Errorf("failed to create reply message: %w", err)
	}

	config.Logger.Info("Reply message created successfully",
		zap.String("messageID", message.ID.String()),
		zap.String("parentMessageID", parentMessageID.String()),
		zap.String("threadID", threadID))

	// Handle file attachments (reuse your existing attachment logic)
	var attachments []*ChatAttachmentSummary
	var attachmentErrors []string

	for _, fileHeader := range files {
		documentRequest := &documents_requests.CreateDocumentRequest{
			CategoryCode:  "CHAT_ATTACHMENT",
			FileName:      fileHeader.Filename,
			CreatedBy:     createdBy,
			ApplicationID: applicationID,
			FileType:      fileHeader.Header.Get("Content-Type"),
		}

		response, err := r.documentSvc.UnifiedCreateDocument(
			tx,
			nil, // You might need to pass context here
			documentRequest,
			nil,
			fileHeader,
		)

		if err != nil {
			errorMsg := fmt.Sprintf("failed to create document for %s: %v", fileHeader.Filename, err)
			attachmentErrors = append(attachmentErrors, errorMsg)
			config.Logger.Error("Failed to create document for chat attachment",
				zap.Error(err),
				zap.String("filename", fileHeader.Filename))
			continue
		}

		if response.Document == nil {
			errorMsg := fmt.Sprintf("document response is nil for %s", fileHeader.Filename)
			attachmentErrors = append(attachmentErrors, errorMsg)
			continue
		}

		chatAttachment := models.ChatAttachment{
			ID:         uuid.New(),
			MessageID:  message.ID,
			DocumentID: response.Document.ID,
		}

		if err := tx.Create(&chatAttachment).Error; err != nil {
			errorMsg := fmt.Sprintf("failed to create chat attachment for %s: %v", fileHeader.Filename, err)
			attachmentErrors = append(attachmentErrors, errorMsg)
			continue
		}

		fileSizeStr := response.Document.FileSize.String()

		attachments = append(attachments, &ChatAttachmentSummary{
			ID:        chatAttachment.ID,
			FileName:  response.Document.FileName,
			FileSize:  fileSizeStr,
			FileType:  string(response.Document.DocumentType),
			MimeType:  response.Document.MimeType,
			FilePath:  response.Document.FilePath,
			CreatedAt: response.Document.CreatedAt.Format(time.RFC3339),
		})
	}

	// Log attachment errors but don't fail
	if len(attachmentErrors) > 0 {
		config.Logger.Warn("Some attachments failed to process for reply",
			zap.Strings("errors", attachmentErrors),
			zap.String("messageID", message.ID.String()))
	}

	// Load the complete message with relationships
	var completeMessage models.ChatMessage
	if err := tx.
		Preload("Sender").
		Preload("Sender.Role").
		Preload("Sender.Department").
		Preload("Attachments").
		Preload("Attachments.Document").
		Preload("Parent").
		Preload("Parent.Sender").
		Where("id = ?", message.ID).
		First(&completeMessage).Error; err != nil {
		return nil, fmt.Errorf("failed to load complete reply message: %w", err)
	}

	// Build attachments if needed
	if len(completeMessage.Attachments) > 0 && len(attachments) == 0 {
		attachments = make([]*ChatAttachmentSummary, len(completeMessage.Attachments))
		for i, attachment := range completeMessage.Attachments {
			fileSizeStr := attachment.Document.FileSize.String()
			attachments[i] = &ChatAttachmentSummary{
				ID:        attachment.ID,
				FileName:  attachment.Document.FileName,
				FileSize:  fileSizeStr,
				FileType:  string(attachment.Document.DocumentType),
				MimeType:  attachment.Document.MimeType,
				FilePath:  attachment.Document.FilePath,
				CreatedAt: attachment.Document.CreatedAt.Format(time.RFC3339),
			}
		}
	}

	// Build parent message summary if exists
	var parentSummary *MessageSummary
	if completeMessage.Parent != nil {
		parentSummary = &MessageSummary{
			ID:      completeMessage.Parent.ID,
			Content: completeMessage.Parent.Content,
			Sender: &UserSummary{
				ID:        completeMessage.Parent.Sender.ID,
				FirstName: completeMessage.Parent.Sender.FirstName,
				LastName:  completeMessage.Parent.Sender.LastName,
				Email:     completeMessage.Parent.Sender.Email,
			},
			CreatedAt: completeMessage.Parent.CreatedAt.Format(time.RFC3339),
		}
	}

	// Convert to enhanced format
	enhancedMessage := &EnhancedChatMessage{
		ID:          completeMessage.ID,
		Content:     completeMessage.Content,
		MessageType: completeMessage.MessageType,
		Status:      completeMessage.Status,
		IsEdited:    completeMessage.IsEdited,
		EditedAt:    utils.FormatTimePointer(completeMessage.EditedAt),
		IsDeleted:   completeMessage.IsDeleted,
		CreatedAt:   completeMessage.CreatedAt.Format(time.RFC3339),
		Sender: &UserSummary{
			ID:        completeMessage.Sender.ID,
			FirstName: completeMessage.Sender.FirstName,
			LastName:  completeMessage.Sender.LastName,
			Email:     completeMessage.Sender.Email,
			Department: utils.DerefString(func() *string {
				if completeMessage.Sender.Department != nil {
					return &completeMessage.Sender.Department.Name
				}
				return nil
			}()),
		},
		ParentID:    completeMessage.ParentID,
		Parent:      parentSummary,
		Attachments: attachments,
	}

	return enhancedMessage, nil
}

// GetMessageThread gets a message and its reply thread
func (r *applicationRepository) GetMessageThread(messageID uuid.UUID) ([]*EnhancedChatMessage, error) {
	var messages []models.ChatMessage

	// Get the parent message and all its replies
	err := r.db.
		Preload("Sender").
		Preload("Sender.Role").
		Preload("Sender.Department").
		Preload("Attachments").
		Preload("Attachments.Document").
		Preload("Parent").
		Preload("Parent.Sender").
		Where("id = ? OR parent_id = ?", messageID, messageID).
		Where("is_deleted = ?", false).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch message thread: %w", err)
	}

	// Convert to enhanced format
	enhancedMessages := make([]*EnhancedChatMessage, len(messages))
	for i, message := range messages {
		// Build attachments
		attachments := make([]*ChatAttachmentSummary, len(message.Attachments))
		for j, attachment := range message.Attachments {
			fileSizeStr := attachment.Document.FileSize.String()
			attachments[j] = &ChatAttachmentSummary{
				ID:        attachment.ID,
				FileName:  attachment.Document.FileName,
				FileSize:  fileSizeStr,
				FileType:  string(attachment.Document.DocumentType),
				MimeType:  attachment.Document.MimeType,
				FilePath:  attachment.Document.FilePath,
				CreatedAt: attachment.Document.CreatedAt.Format(time.RFC3339),
			}
		}

		// Build parent summary if exists
		var parentSummary *MessageSummary
		if message.Parent != nil {
			parentSummary = &MessageSummary{
				ID:      message.Parent.ID,
				Content: message.Parent.Content,
				Sender: &UserSummary{
					ID:        message.Parent.Sender.ID,
					FirstName: message.Parent.Sender.FirstName,
					LastName:  message.Parent.Sender.LastName,
					Email:     message.Parent.Sender.Email,
				},
				CreatedAt: message.Parent.CreatedAt.Format(time.RFC3339),
			}
		}

		enhancedMessages[i] = &EnhancedChatMessage{
			ID:          message.ID,
			Content:     message.Content,
			MessageType: message.MessageType,
			Status:      message.Status,
			IsEdited:    message.IsEdited,
			EditedAt:    utils.FormatTimePointer(message.EditedAt),
			IsDeleted:   message.IsDeleted,
			CreatedAt:   message.CreatedAt.Format(time.RFC3339),
			Sender: &UserSummary{
				ID:        message.Sender.ID,
				FirstName: message.Sender.FirstName,
				LastName:  message.Sender.LastName,
				Email:     message.Sender.Email,
				Department: utils.DerefString(func() *string {
					if message.Sender.Department != nil {
						return &message.Sender.Department.Name
					}
					return nil
				}()),
			},
			ParentID:    message.ParentID,
			Parent:      parentSummary,
			Attachments: attachments,
		}
	}

	return enhancedMessages, nil
}

// repositories/chat_repository.go
package repositories

import (
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
		_, err := r.documentSvc.UnifiedCreateDocument(
			tx,
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

		// Since we can't get the document ID from the response, we need to query for it
		// This assumes your document service creates the document in the same transaction
		document, err := r.findLatestDocumentByFileName(tx, fileHeader.Filename, createdBy)
		if err != nil {
			errorMsg := fmt.Sprintf("failed to find created document for %s: %v", fileHeader.Filename, err)
			attachmentErrors = append(attachmentErrors, errorMsg)
			config.Logger.Error("Failed to find created document",
				zap.Error(err),
				zap.String("filename", fileHeader.Filename))
			continue
		}

		// Create chat attachment linking to the document
		chatAttachment := models.ChatAttachment{
			ID:         uuid.New(),
			MessageID:  message.ID,
			DocumentID: document.ID,
		}

		if err := tx.Create(&chatAttachment).Error; err != nil {
			errorMsg := fmt.Sprintf("failed to create chat attachment for %s: %v", fileHeader.Filename, err)
			attachmentErrors = append(attachmentErrors, errorMsg)
			config.Logger.Error("Failed to create chat attachment",
				zap.Error(err),
				zap.String("documentID", document.ID.String()),
				zap.String("filename", fileHeader.Filename))
			continue
		}

		// FIXED: Convert file size to string directly (decimal.Decimal has String() method)
		fileSizeStr := document.FileSize.String()

		attachments = append(attachments, &ChatAttachmentSummary{
			ID:        chatAttachment.ID,
			FileName:  document.FileName,
			FileSize:  fileSizeStr,
			FileType:  string(document.DocumentType),
			MimeType:  document.MimeType,
			FilePath:  document.FilePath,
			CreatedAt: document.CreatedAt.Format(time.RFC3339),
		})

		config.Logger.Info("Chat attachment created successfully",
			zap.String("filename", fileHeader.Filename),
			zap.String("documentID", document.ID.String()),
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

	// Get the complete message with preloaded relationships
	var completeMessage models.ChatMessage
	if err := r.db.
		Preload("Sender").
		Preload("Sender.Role").
		Preload("Sender.Department").
		Preload("Attachments").
		Preload("Attachments.Document").
		Where("id = ?", message.ID).
		First(&completeMessage).Error; err != nil {
		return nil, fmt.Errorf("failed to load complete message: %w", err)
	}

	// Build attachments from preloaded data (in case some were created outside the transaction)
	if len(completeMessage.Attachments) > 0 && len(attachments) == 0 {
		attachments = make([]*ChatAttachmentSummary, len(completeMessage.Attachments))
		for i, attachment := range completeMessage.Attachments {
			// FIXED: Use direct String() method for decimal.Decimal
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

// findLatestDocumentByFileName finds the most recent document by filename and creator
func (r *applicationRepository) findLatestDocumentByFileName(tx *gorm.DB, fileName, createdBy string) (*models.Document, error) {
	var document models.Document
	if err := tx.
		Where("file_name = ? AND created_by = ?", fileName, createdBy).
		Order("created_at DESC").
		First(&document).Error; err != nil {
		return nil, fmt.Errorf("failed to find document: %w", err)
	}
	return &document, nil
}

// GetChatMessagesWithPreload gets messages with all relationships preloaded
func (r *applicationRepository) GetChatMessagesWithPreload(threadID string, limit, offset int) ([]*EnhancedChatMessage, int, error) {
	var messages []models.ChatMessage

	// Get total count
	var total int64
	if err := r.db.Model(&models.ChatMessage{}).
		Where("thread_id = ? AND is_deleted = ?", threadID, false).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated messages with ALL relationships preloaded in one query
	if err := r.db.
		Preload("Sender").
		Preload("Sender.Role").
		Preload("Sender.Department").
		Preload("Attachments").
		Preload("Attachments.Document").
		Where("thread_id = ? AND is_deleted = ?", threadID, false).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	// Convert to enhanced format - no additional DB queries needed
	enhancedMessages := make([]*EnhancedChatMessage, len(messages))
	for i, message := range messages {
		// Build attachments from preloaded data
		attachments := make([]*ChatAttachmentSummary, len(message.Attachments))
		for j, attachment := range message.Attachments {
			// FIXED: Use direct String() method for decimal.Decimal
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

		// Messages are returned in DESC order, but we want to display in ASC order
		// So we'll reverse them in the frontend
		enhancedMessages[len(messages)-1-i] = &EnhancedChatMessage{
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
			Attachments: attachments,
		}
	}

	return enhancedMessages, int(total), nil
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

// AddParticipantToThread adds a user to a chat thread
func (r *applicationRepository) AddParticipantToThread(
	threadID uuid.UUID,
	userID uuid.UUID,
	role models.ParticipantRole,
	addedBy string,
) error {

	participant := models.ChatParticipant{
		ID:       uuid.New(),
		ThreadID: threadID,
		UserID:   userID,
		Role:     role,
		IsActive: true,
		AddedBy:  addedBy,
		AddedAt:  time.Now(),
	}

	return r.db.Create(&participant).Error
}

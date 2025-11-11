package controllers

import (
	"fmt"
	"mime/multipart"
	"time"
	applicationRepositories "town-planning-backend/applications/repositories"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	documents_requests "town-planning-backend/documents/requests"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RaiseIssue handles raising an issue for an application with chat thread creation and file attachments
func (ac *ApplicationController) RaiseIssueController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Parse multipart form instead of JSON
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid form data",
			"error":   err.Error(),
		})
	}

	// Extract form values
	request := RaiseIssueRequest{
		Title:                   getFormValue(form, "title"),
		Description:             getFormValue(form, "description"),
		Priority:                getFormValue(form, "priority"),
		Category:                getFormValuePtr(form, "category"),
		AssignmentType:          models.IssueAssignmentType(getFormValue(form, "assignment_type")),
		AssignedToUserID:        getUUIDPtrFromForm(form, "assigned_to_user_id"),
		AssignedToGroupMemberID: getUUIDPtrFromForm(form, "assigned_to_group_member_id"),
	}

	// Get uploaded files
	files := form.File["attachments"]

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

	user, err := ac.UserRepo.GetUserByID(userUUID.String())
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Please log out and log in again",
		})
	}

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

	// ========================================
	// PROCESS FILE ATTACHMENTS IN CONTROLLER
	// ========================================
	var attachmentDocumentIDs []uuid.UUID
	if len(files) > 0 {
		attachmentDocumentIDs, err = ac.processChatAttachments(tx, c, files, user.Email, applicationID)
		if err != nil {
			tx.Rollback()
			config.Logger.Error("Failed to process chat attachments",
				zap.Error(err),
				zap.String("applicationID", applicationID))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to process file attachments",
				"error":   err.Error(),
			})
		}
	}

	// Process issue creation with chat thread and file attachments
	issue, chatThread, initialMessage, err := ac.ApplicationRepo.RaiseApplicationIssueWithChatAndAttachments(
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
		attachmentDocumentIDs, // Pass document IDs instead of file headers
		user.Email,
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

	// For the sake of broadcasting the message we need to create the EnhancedChatMessage
	enhancedMessage := &applicationRepositories.EnhancedChatMessage{
		ID:          initialMessage.ID, // Generate a new message ID
		Content:     initialMessage.Content,
		MessageType: initialMessage.MessageType,
		Status:      "SENT", // Or the appropriate status
		CreatedAt:   initialMessage.CreatedAt.Format(time.RFC3339),
		Sender: &applicationRepositories.UserSummary{
			ID:        userUUID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
		},
	}

	// Increment unread counts for all participants except sender
	if err := ac.incrementUnreadCounts(tx, chatThread.ID.String(), userUUID); err != nil {
		config.Logger.Warn("Failed to increment unread counts",
			zap.Error(err),
			zap.String("threadID", chatThread.ID.String()))
	}

	// Now broadcast the message
	ac.broadcastNewMessage(chatThread.ID.String(), *enhancedMessage, userUUID)

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

	config.Logger.Info("Issue raised successfully with chat thread and attachments",
		zap.String("applicationID", applicationID),
		zap.String("userID", userUUID.String()),
		zap.String("issueID", issue.ID.String()),
		zap.String("chatThreadID", chatThread.ID.String()),
		zap.String("assignmentType", string(request.AssignmentType)),
		zap.Int("attachmentCount", len(attachmentDocumentIDs)))

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

// processChatAttachments processes file attachments and returns document IDs
func (ac *ApplicationController) processChatAttachments(
	tx *gorm.DB,
	c *fiber.Ctx,
	files []*multipart.FileHeader,
	createdBy string,
	applicationID string,
) ([]uuid.UUID, error) {

	var documentIDs []uuid.UUID

	appID, err := uuid.Parse(applicationID)
	if err != nil {
		return nil, fmt.Errorf("invalid application ID: %w", err)
	}

	for _, fileHeader := range files {
		// Create document request for chat attachment
		documentRequest := &documents_requests.CreateDocumentRequest{
			CategoryCode:  "CHAT_ATTACHMENT", // Make sure this category exists
			FileName:      fileHeader.Filename,
			CreatedBy:     createdBy,
			ApplicationID: &appID,
			FileType:      fileHeader.Header.Get("Content-Type"),
		}

		// Use your existing document service to create the document
		response, err := ac.DocumentSvc.UnifiedCreateDocument(
			tx,
			c,
			documentRequest,
			nil, // No file content bytes, we'll use the multipart file
			fileHeader,
		)

		if err != nil {
			config.Logger.Error("Failed to create document for chat attachment",
				zap.Error(err),
				zap.String("filename", fileHeader.Filename))
			// Continue with other files instead of failing the entire operation
			continue
		}

		documentIDs = append(documentIDs, response.Document.ID)

		config.Logger.Info("Chat attachment document created successfully",
			zap.String("filename", fileHeader.Filename),
			zap.String("documentID", response.Document.ID.String()))
	}

	if len(documentIDs) == 0 && len(files) > 0 {
		return nil, fmt.Errorf("failed to process any of the %d file attachments", len(files))
	}

	return documentIDs, nil
}

func getFormValuePtr(form *multipart.Form, key string) *string {
	if values, exists := form.Value[key]; exists && len(values) > 0 && values[0] != "" {
		return &values[0]
	}
	return nil
}

func getUUIDPtrFromForm(form *multipart.Form, key string) *uuid.UUID {
	if values, exists := form.Value[key]; exists && len(values) > 0 && values[0] != "" {
		if id, err := uuid.Parse(values[0]); err == nil {
			return &id
		}
	}
	return nil
}

// controllers/chat_controller.go

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

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UnifiedParticipantController handles both single and bulk participant operations
func (ac *ApplicationController) UnifiedParticipantController(c *fiber.Ctx) error {
	threadID := c.Params("threadId")
	if threadID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Thread ID is required",
		})
	}

	var request requests.UnifiedParticipantRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Get current user
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	currentUserID := payload.UserID

	// Get user details for audit
	user, err := ac.UserRepo.GetUserByID(currentUserID.String())
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	// Validate operation type
	if request.Operation == "" {
		// Auto-detect operation type based on provided fields
		if request.UserID != uuid.Nil && len(request.Participants) == 0 && len(request.UserIDs) == 0 {
			request.Operation = "add_single"
		} else if len(request.Participants) > 0 && request.UserID == uuid.Nil && len(request.UserIDs) == 0 {
			request.Operation = "add_bulk"
		} else if len(request.UserIDs) > 0 && request.UserID == uuid.Nil && len(request.Participants) == 0 {
			request.Operation = "remove_bulk"
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Cannot determine operation type. Please specify 'operation' field or provide clear input",
			})
		}
	}

	// Validate request based on operation type
	if err := validateParticipantRequest(request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction for participant operation",
			zap.Error(tx.Error),
			zap.String("threadID", threadID),
			zap.String("operation", request.Operation))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Internal server error",
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic during participant operation, rolling back",
				zap.Any("panic", r),
				zap.String("threadID", threadID))
			panic(r)
		}
	}()

	// Check SPECIFIC permission for the operation
	var requiredPermission string
	switch request.Operation {
	case "add_single", "add_bulk":
		requiredPermission = "add"
	case "remove_single", "remove_bulk":
		requiredPermission = "remove"
	default:
		requiredPermission = "any"
	}

	canManage, err := ac.ApplicationRepo.CanUserManageParticipants(threadID, currentUserID, requiredPermission)
	if err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to check permissions",
		})
	}

	if !canManage {
		tx.Rollback()
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "You don't have permission to manage participants in this thread",
		})
	}

	// Parse thread ID
	threadUUID, err := uuid.Parse(threadID)
	if err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid thread ID",
		})
	}

	// Execute the requested operation
	var result interface{}
	var message string

	// log the request
	config.Logger.Info("Participant operation request",
		zap.String("threadID", threadID),
		zap.String("operation", request.Operation),
		zap.Any("request", request))

	//log in terminal
	fmt.Println("Participant operation request", request)

	switch request.Operation {
	case "add_single":
		result, message, err = ac.handleAddSingleParticipant(tx, threadUUID, request, user)
	case "add_bulk":
		result, message, err = ac.handleAddBulkParticipants(tx, threadUUID, request, user)
	case "remove_single":
		result, message, err = ac.handleRemoveSingleParticipant(tx, threadUUID, request, user)
	case "remove_bulk":
		result, message, err = ac.handleRemoveBulkParticipants(tx, threadUUID, request, user)
	default:
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid operation type",
		})
	}

	if err != nil {
		tx.Rollback()
		return handleParticipantError(err, request.Operation)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction for participant operation",
			zap.Error(err),
			zap.String("threadID", threadID),
			zap.String("operation", request.Operation))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to complete operation",
		})
	}

	config.Logger.Info("Participant operation completed successfully",
		zap.String("threadID", threadID),
		zap.String("operation", request.Operation),
		zap.String("performedBy", currentUserID.String()))

	return c.JSON(fiber.Map{
		"success": true,
		"message": message,
		"data":    result,
	})
}

// controllers/chat_controller.go

func (ac *ApplicationController) handleAddSingleParticipant(
	tx *gorm.DB,
	threadUUID uuid.UUID,
	request requests.UnifiedParticipantRequest,
	addedBy *models.User,
) (interface{}, string, error) {

	// Verify target user exists
	targetUser, err := ac.UserRepo.GetUserByID(request.UserID.String())
	if err != nil {
		return nil, "", fmt.Errorf("target user not found")
	}

	// Set defaults if not provided
	role := request.Role
	if role == "" {
		role = models.ParticipantRoleMember
	}

	// Use provided permissions or set smart defaults
	canInvite := getBoolOrDefault(request.CanInvite, true)
	canRemove := getBoolOrDefault(request.CanRemove, false)
	canManage := getBoolOrDefault(request.CanManage, false)

	// Add participant with specific permissions
	if err := ac.ApplicationRepo.AddParticipantToThread(
		tx,
		threadUUID,
		request.UserID,
		role,
		addedBy.ID.String(),
		canInvite,
		canRemove,
		canManage,
	); err != nil {
		return nil, "", err
	}

	// ==================== CREATE SINGLE PROFESSIONAL SYSTEM MESSAGE ====================
	messageContent := ac.formatAddParticipantMessage(addedBy, targetUser, canInvite, canRemove, canManage)

	systemMessage := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    threadUUID,
		SenderID:    addedBy.ID,
		Content:     messageContent,
		MessageType: models.MessageTypeSystem,
		Status:      models.MessageStatusSent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := tx.Create(&systemMessage).Error; err != nil {
		config.Logger.Warn("Failed to create participant added message", zap.Error(err))
	} else {
		// Increment unread counts and broadcast
		if err := ac.incrementUnreadCounts(tx, threadUUID.String(), addedBy.ID); err != nil {
			config.Logger.Warn("Failed to increment unread counts for participant message", zap.Error(err))
		}
		enhancedMessage := ac.createEnhancedMessage(systemMessage, *addedBy)
		ac.broadcastNewMessage(threadUUID.String(), *enhancedMessage, addedBy.ID)
	}

	result := fiber.Map{
		"thread_id": threadUUID,
		"user_id":   request.UserID,
		"role":      role,
		"permissions": fiber.Map{
			"can_invite": canInvite,
			"can_remove": canRemove,
			"can_manage": canManage,
		},
	}

	return result, "Participant added successfully", nil
}

func (ac *ApplicationController) handleAddBulkParticipants(
	tx *gorm.DB,
	threadUUID uuid.UUID,
	request requests.UnifiedParticipantRequest,
	addedBy *models.User,
) (interface{}, string, error) {

	// Verify all users exist and collect their details
	addedUsers := make([]*models.User, 0, len(request.Participants))
	for _, participant := range request.Participants {
		user, err := ac.UserRepo.GetUserByID(participant.UserID.String())
		if err != nil {
			return nil, "", fmt.Errorf("user %s not found", participant.UserID)
		}
		addedUsers = append(addedUsers, user)
	}

	// Convert to repository format and add participants
	participantReqs := make([]requests.ParticipantRequest, len(request.Participants))
	for i, p := range request.Participants {
		role := p.Role
		if role == "" {
			role = models.ParticipantRoleMember
		}
		participantReqs[i] = requests.ParticipantRequest{
			UserID:    p.UserID,
			Role:      role,
			CanInvite: p.CanInvite,
			CanRemove: p.CanRemove,
			CanManage: p.CanManage,
		}
	}

	// Add multiple participants
	createdParticipants, err := ac.ApplicationRepo.AddMultipleParticipantsToThread(
		tx,
		threadUUID,
		participantReqs,
		addedBy,
	)
	if err != nil {
		return nil, "", err
	}

	// ==================== CREATE SINGLE PROFESSIONAL BULK ADD MESSAGE ====================
	messageContent := ac.formatBulkAddParticipantsMessage(addedBy, addedUsers)

	systemMessage := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    threadUUID,
		SenderID:    addedBy.ID,
		Content:     messageContent,
		MessageType: models.MessageTypeSystem,
		Status:      models.MessageStatusSent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := tx.Create(&systemMessage).Error; err != nil {
		config.Logger.Warn("Failed to create bulk participant added message", zap.Error(err))
	} else {
		// Increment unread counts and broadcast
		if err := ac.incrementUnreadCounts(tx, threadUUID.String(), addedBy.ID); err != nil {
			config.Logger.Warn("Failed to increment unread counts for bulk add message", zap.Error(err))
		}
		enhancedMessage := ac.createEnhancedMessage(systemMessage, *addedBy)
		ac.broadcastNewMessage(threadUUID.String(), *enhancedMessage, addedBy.ID)
	}

	// Transform response
	participantResponses := make([]fiber.Map, len(createdParticipants))
	for i, participant := range createdParticipants {
		participantResponses[i] = fiber.Map{
			"user_id": participant.UserID,
			"role":    participant.Role,
		}
	}

	result := fiber.Map{
		"thread_id":    threadUUID,
		"added_count":  len(createdParticipants),
		"participants": participantResponses,
	}

	return result, fmt.Sprintf("%d participants added successfully", len(createdParticipants)), nil
}

func (ac *ApplicationController) handleRemoveSingleParticipant(
	tx *gorm.DB,
	threadUUID uuid.UUID,
	request requests.UnifiedParticipantRequest,
	removedBy *models.User,
) (interface{}, string, error) {

	// Get thread info to protect the creator
	var thread models.ChatThread
	if err := tx.Where("id = ?", threadUUID).First(&thread).Error; err != nil {
		return nil, "", err
	}

	// Prevent removing thread creator
	if request.UserID == thread.CreatedByUserID {
		return nil, "", fmt.Errorf("cannot remove thread creator")
	}

	// Get user who is being removed
	removedUser, err := ac.UserRepo.GetUserByID(request.UserID.String())
	if err != nil {
		return nil, "", fmt.Errorf("failed to get removed user details")
	}

	// Remove participant (NO MESSAGE CREATION IN REPOSITORY)
	if err := ac.ApplicationRepo.RemoveParticipantFromThread(
		tx,
		threadUUID,
		request.UserID,
		removedBy,
	); err != nil {
		return nil, "", err
	}

	// ==================== CREATE SINGLE PROFESSIONAL REMOVAL MESSAGE ====================
	messageContent := ac.formatRemoveParticipantMessage(removedBy, removedUser)

	systemMessage := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    threadUUID,
		SenderID:    removedBy.ID,
		Content:     messageContent,
		MessageType: models.MessageTypeSystem,
		Status:      models.MessageStatusSent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := tx.Create(&systemMessage).Error; err != nil {
		config.Logger.Warn("Failed to create participant removed message", zap.Error(err))
	} else {
		// Increment unread counts and broadcast
		if err := ac.incrementUnreadCounts(tx, threadUUID.String(), removedBy.ID); err != nil {
			config.Logger.Warn("Failed to increment unread counts for removal message", zap.Error(err))
		}
		enhancedMessage := ac.createEnhancedMessage(systemMessage, *removedBy)
		ac.broadcastNewMessage(threadUUID.String(), *enhancedMessage, removedBy.ID)
	}

	result := fiber.Map{
		"thread_id": threadUUID,
		"user_id":   request.UserID,
	}

	return result, "Participant removed successfully", nil
}

func (ac *ApplicationController) handleRemoveBulkParticipants(
	tx *gorm.DB,
	threadUUID uuid.UUID,
	request requests.UnifiedParticipantRequest,
	removedBy *models.User,
) (interface{}, string, error) {

	// Get removed users details for the message
	removedUsers := make([]*models.User, 0, len(request.UserIDs))
	for _, userID := range request.UserIDs {
		user, err := ac.UserRepo.GetUserByID(userID.String())
		if err != nil {
			config.Logger.Warn("Failed to get removed user details", zap.String("userID", userID.String()))
			continue
		}
		removedUsers = append(removedUsers, user)
	}

	// Remove multiple participants (NO MESSAGE CREATION IN REPOSITORY)
	removedCount, err := ac.ApplicationRepo.RemoveMultipleParticipantsFromThread(
		tx,
		threadUUID,
		request.UserIDs,
		removedBy,
	)
	if err != nil {
		return nil, "", err
	}

	// ==================== CREATE SINGLE PROFESSIONAL BULK REMOVAL MESSAGE ====================
	messageContent := ac.formatBulkRemoveParticipantsMessage(removedBy, removedUsers)

	systemMessage := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    threadUUID,
		SenderID:    removedBy.ID,
		Content:     messageContent,
		MessageType: models.MessageTypeSystem,
		Status:      models.MessageStatusSent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := tx.Create(&systemMessage).Error; err != nil {
		config.Logger.Warn("Failed to create bulk removal message", zap.Error(err))
	} else {
		// Increment unread counts and broadcast
		if err := ac.incrementUnreadCounts(tx, threadUUID.String(), removedBy.ID); err != nil {
			config.Logger.Warn("Failed to increment unread counts for bulk removal message", zap.Error(err))
		}
		enhancedMessage := ac.createEnhancedMessage(systemMessage, *removedBy)
		ac.broadcastNewMessage(threadUUID.String(), *enhancedMessage, removedBy.ID)
	}

	result := fiber.Map{
		"thread_id":     threadUUID,
		"removed_count": removedCount,
		"user_ids":      request.UserIDs,
		"removed_by":    removedBy.ID,
	}

	return result, fmt.Sprintf("%d participants removed successfully", removedCount), nil
}

// Validation and helper functions
func validateParticipantRequest(request requests.UnifiedParticipantRequest) error {
	switch request.Operation {
	case "add_single":
		if request.UserID == uuid.Nil {
			return fmt.Errorf("user_id is required for single add operation")
		}
	case "add_bulk":
		if len(request.Participants) == 0 {
			return fmt.Errorf("participants array is required for bulk add operation")
		}
		for i, participant := range request.Participants {
			if participant.UserID == uuid.Nil {
				return fmt.Errorf("participants[%d].user_id is required", i)
			}
		}
	case "remove_single":
		if request.UserID == uuid.Nil {
			return fmt.Errorf("user_id is required for single remove operation")
		}
	case "remove_bulk":
		if len(request.UserIDs) == 0 {
			return fmt.Errorf("user_ids array is required for bulk remove operation")
		}
		for i, userID := range request.UserIDs {
			if userID == uuid.Nil {
				return fmt.Errorf("user_ids[%d] is invalid", i)
			}
		}
	default:
		return fmt.Errorf("invalid operation type: %s", request.Operation)
	}
	return nil
}

func handleParticipantError(err error, operation string) *fiber.Error {
	errorMsg := err.Error()

	switch {
	case strings.Contains(errorMsg, "already a participant"):
		return fiber.NewError(fiber.StatusConflict, "User is already a participant in this thread")
	case strings.Contains(errorMsg, "cannot remove thread owner"):
		return fiber.NewError(fiber.StatusForbidden, "Cannot remove thread owner")
	case strings.Contains(errorMsg, "participant not found"):
		return fiber.NewError(fiber.StatusNotFound, "Participant not found in this thread")
	case strings.Contains(errorMsg, "not found"):
		return fiber.NewError(fiber.StatusBadRequest, errorMsg)
	default:
		config.Logger.Error("Participant operation failed",
			zap.String("operation", operation),
			zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("Failed to %s participant(s)", operation))
	}
}

// GetThreadParticipantsController gets all participants for a thread (unchanged)
func (ac *ApplicationController) GetThreadParticipantsController(c *fiber.Ctx) error {
	threadID := c.Params("threadId")
	if threadID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Thread ID is required",
		})
	}

	participants, err := ac.ApplicationRepo.GetThreadParticipants(threadID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch participants",
			"error":   err.Error(),
		})
	}

	// Transform response
	participantResponses := make([]fiber.Map, len(participants))
	for i, participant := range participants {
		participantResponses[i] = fiber.Map{
			"id":        participant.ID,
			"user_id":   participant.UserID,
			"role":      participant.Role,
			"is_active": participant.IsActive,
			"added_at":  participant.AddedAt,
			"added_by":  participant.AddedBy,
			"user": fiber.Map{
				"id":         participant.User.ID,
				"first_name": participant.User.FirstName,
				"last_name":  participant.User.LastName,
				"email":      participant.User.Email,
				"department": participant.User.Department.Name,
				"role":       participant.User.Role.Name,
			},
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"participants": participantResponses,
			"total_count":  len(participants),
		},
	})
}

// Helper function for default boolean values
func getBoolOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// Helper to create enhanced message for broadcasting
func (ac *ApplicationController) createEnhancedMessage(message models.ChatMessage, sender models.User) *applicationRepositories.EnhancedChatMessage {
	return &applicationRepositories.EnhancedChatMessage{
		ID:          message.ID,
		Content:     message.Content,
		MessageType: message.MessageType,
		Status:      message.Status,
		CreatedAt:   message.CreatedAt.Format(time.RFC3339),
		Sender: &applicationRepositories.UserSummary{
			ID:        sender.ID,
			FirstName: sender.FirstName,
			LastName:  sender.LastName,
			Email:     sender.Email,
			Department: func() string {
				if sender.Department != nil {
					return sender.Department.Name
				}
				return ""
			}(),
		},
		ParentID:    nil,
		Attachments: nil,
	}
}

// controllers/chat_controller.go

// Professional message formatting functions
func (ac *ApplicationController) formatAddParticipantMessage(addedBy, targetUser *models.User, canInvite, canRemove, canManage bool) string {
	baseMessage := fmt.Sprintf("%s %s added %s %s to the conversation",
		addedBy.FirstName, addedBy.LastName,
		targetUser.FirstName, targetUser.LastName)

	// Add permission details if non-default
	if canInvite || canRemove || canManage {
		permissions := []string{}
		if canInvite {
			permissions = append(permissions, "invite")
		}
		if canRemove {
			permissions = append(permissions, "remove")
		}
		if canManage {
			permissions = append(permissions, "manage permissions")
		}

		if len(permissions) > 0 {
			baseMessage += fmt.Sprintf(" with %s permissions", strings.Join(permissions, "/"))
		}
	}

	return baseMessage
}

func (ac *ApplicationController) formatBulkAddParticipantsMessage(addedBy *models.User, addedUsers []*models.User) string {
	if len(addedUsers) == 1 {
		return fmt.Sprintf("%s %s added %s %s to the conversation",
			addedBy.FirstName, addedBy.LastName,
			addedUsers[0].FirstName, addedUsers[0].LastName)
	} else if len(addedUsers) <= 3 {
		userNames := make([]string, len(addedUsers))
		for i, user := range addedUsers {
			userNames[i] = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}
		return fmt.Sprintf("%s %s added %s to the conversation",
			addedBy.FirstName, addedBy.LastName,
			strings.Join(userNames, ", "))
	} else {
		return fmt.Sprintf("%s %s added %d participants to the conversation",
			addedBy.FirstName, addedBy.LastName,
			len(addedUsers))
	}
}

func (ac *ApplicationController) formatRemoveParticipantMessage(removedBy, removedUser *models.User) string {
	return fmt.Sprintf("%s %s removed %s %s from the conversation",
		removedBy.FirstName, removedBy.LastName,
		removedUser.FirstName, removedUser.LastName)
}

func (ac *ApplicationController) formatBulkRemoveParticipantsMessage(removedBy *models.User, removedUsers []*models.User) string {
	if len(removedUsers) == 1 {
		return fmt.Sprintf("%s %s removed %s %s from the conversation",
			removedBy.FirstName, removedBy.LastName,
			removedUsers[0].FirstName, removedUsers[0].LastName)
	} else if len(removedUsers) <= 3 {
		userNames := make([]string, len(removedUsers))
		for i, user := range removedUsers {
			userNames[i] = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}
		return fmt.Sprintf("%s %s removed %s from the conversation",
			removedBy.FirstName, removedBy.LastName,
			strings.Join(userNames, ", "))
	} else {
		return fmt.Sprintf("%s %s removed %d participants from the conversation",
			removedBy.FirstName, removedBy.LastName,
			len(removedUsers))
	}
}

// controllers/chat_controller.go

package controllers

import (
	"fmt"
	"strings"
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

	// Check if current user can manage participants
	canManage, err := ac.ApplicationRepo.CanUserManageParticipants(threadID, currentUserID)
	if err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to check permissions",
			"error":   err.Error(),
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

	switch request.Operation {
	case "add_single":
		result, message, err = ac.handleAddSingleParticipant(tx, threadUUID, request, user.Email)
	case "add_bulk":
		result, message, err = ac.handleAddBulkParticipants(tx, threadUUID, request, user.Email)
	case "remove_single":
		result, message, err = ac.handleRemoveSingleParticipant(tx, threadUUID, request, user.Email)
	case "remove_bulk":
		result, message, err = ac.handleRemoveBulkParticipants(tx, threadUUID, request, user.Email)
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

// Handler functions for different operations
func (ac *ApplicationController) handleAddSingleParticipant(
	tx *gorm.DB,
	threadUUID uuid.UUID,
	request requests.UnifiedParticipantRequest,
	addedBy string,
) (interface{}, string, error) {

	// Verify the target user exists
	_, err := ac.UserRepo.GetUserByID(request.UserID.String())
	if err != nil {
		return nil, "", fmt.Errorf("target user not found")
	}

	// Set default role if not provided
	role := request.Role
	if role == "" {
		role = models.ParticipantRoleMember
	}

	// Add participant
	if err := ac.ApplicationRepo.AddParticipantToThread(
		tx,
		threadUUID,
		request.UserID,
		role,
		addedBy,
	); err != nil {
		return nil, "", err
	}

	result := fiber.Map{
		"thread_id": threadUUID,
		"user_id":   request.UserID,
		"role":      role,
	}

	return result, "Participant added successfully", nil
}

func (ac *ApplicationController) handleAddBulkParticipants(
	tx *gorm.DB,
	threadUUID uuid.UUID,
	request requests.UnifiedParticipantRequest,
	addedBy string,
) (interface{}, string, error) {

	// Verify all users exist
	for _, participant := range request.Participants {
		_, err := ac.UserRepo.GetUserByID(participant.UserID.String())
		if err != nil {
			return nil, "", fmt.Errorf("user %s not found", participant.UserID)
		}
	}

	// Convert to repository format
	participantReqs := make([]requests.ParticipantRequest, len(request.Participants))
	for i, p := range request.Participants {
		role := p.Role
		if role == "" {
			role = models.ParticipantRoleMember
		}
		participantReqs[i] = requests.ParticipantRequest{
			UserID: p.UserID,
			Role:   role,
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
	removedBy string,
) (interface{}, string, error) {

	// Remove participant
	if err := ac.ApplicationRepo.RemoveParticipantFromThread(
		tx,
		threadUUID,
		request.UserID,
		removedBy,
	); err != nil {
		return nil, "", err
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
	removedBy string,
) (interface{}, string, error) {

	// Remove multiple participants
	removedCount, err := ac.ApplicationRepo.RemoveMultipleParticipantsFromThread(
		tx,
		threadUUID,
		request.UserIDs,
		removedBy,
	)
	if err != nil {
		return nil, "", err
	}

	result := fiber.Map{
		"thread_id":     threadUUID,
		"removed_count": removedCount,
		"user_ids":      request.UserIDs,
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

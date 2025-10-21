package controllers

import (
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CreateApprovalGroupRequest struct {
	Name                 string                       `json:"name"`
	Description          *string                      `json:"description"`
	Type                 models.ApprovalGroupType     `json:"type"`
	RequiresAllApprovals bool                         `json:"requires_all_approvals"`
	MinimumApprovals     int                          `json:"minimum_approvals"`
	AutoAssignBackups    bool                         `json:"auto_assign_backups"`
	IsActive             bool                         `json:"is_active"`
	CreatedBy            string                       `json:"created_by"`
	Members              []ApprovalGroupMemberRequest `json:"members"`
}

type ApprovalGroupMemberRequest struct {
	UserID             uuid.UUID                 `json:"user_id"`
	Role               models.MemberRole         `json:"role"`
	CanRaiseIssues     bool                      `json:"can_raise_issues"`
	CanApprove         bool                      `json:"can_approve"`
	CanReject          bool                      `json:"can_reject"`
	ReviewOrder        int                       `json:"review_order"`
	BackupPriority     int                       `json:"backup_priority"`
	AvailabilityStatus models.AvailabilityStatus `json:"availability_status"`
	AutoReassign       bool                      `json:"auto_reassign"`
	AddedBy            string                    `json:"added_by"`
	// NEW: Flag to mark as final approver
	IsFinalApprover bool `json:"is_final_approver"`
}

func (ac *ApplicationController) CreateApprovalGroupWithMembers(c *fiber.Ctx) error {
	var request CreateApprovalGroupRequest

	// Parse incoming JSON payload
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request payload",
			"error":   err.Error(),
		})
	}

	// Validate required fields
	if request.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Group name is required",
		})
	}

	if request.CreatedBy == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Created by field is required",
		})
	}

	if len(request.Members) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "At least one member is required",
		})
	}

	// Validate UUID formats and count final approvers
	finalApproverCount := 0
	for i, member := range request.Members {
		if member.UserID == uuid.Nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf("Invalid user ID for member at index %d", i),
			})
		}
		if member.IsFinalApprover {
			finalApproverCount++
		}
	}

	// Validate exactly one final approver
	if finalApproverCount != 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Exactly one member must be designated as final approver. Found %d", finalApproverCount),
			"error":   "Invalid final approver count",
		})
	}

	// Map DTO to GORM model for ApprovalGroup
	approvalGroup := models.ApprovalGroup{
		Name:                 request.Name,
		Description:          request.Description,
		Type:                 request.Type,
		RequiresAllApprovals: request.RequiresAllApprovals,
		MinimumApprovals:     request.MinimumApprovals,
		AutoAssignBackups:    request.AutoAssignBackups,
		IsActive:             request.IsActive,
		CreatedBy:            request.CreatedBy,
	}

	// Map members - now including final approver flag
	for _, memberReq := range request.Members {
		member := models.ApprovalGroupMember{
			UserID:             memberReq.UserID,
			Role:               memberReq.Role,
			CanRaiseIssues:     memberReq.CanRaiseIssues,
			CanApprove:         memberReq.CanApprove,
			CanReject:          memberReq.CanReject,
			ReviewOrder:        memberReq.ReviewOrder,
			BackupPriority:     memberReq.BackupPriority,
			AvailabilityStatus: memberReq.AvailabilityStatus,
			AutoReassign:       memberReq.AutoReassign,
			IsActive:           true,
			AddedBy:            memberReq.AddedBy,
			// NEW: Set final approver flag
			IsFinalApprover: memberReq.IsFinalApprover,
		}
		approvalGroup.Members = append(approvalGroup.Members, member)
	}

	// --- Start Database Transaction ---
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not start database transaction",
			"error":   tx.Error.Error(),
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic detected, rolling back transaction", zap.Any("panic_reason", r))
			panic(r)
		}
	}()

	// Save the approval group to the database (includes all members with final approver)
	createdGroup, err := ac.ApplicationRepo.CreateApprovalGroup(tx, &approvalGroup)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to create approval group in database",
			zap.Error(err),
			zap.String("groupName", approvalGroup.Name))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Something went wrong while creating approval group in the database",
			"error":   err.Error(),
		})
	}

	// --- Commit Database Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	// Fetch the complete group with members for response
	completeGroup, err := ac.ApplicationRepo.GetApprovalGroupWithMembers(ac.DB, createdGroup.ID.String())
	if err != nil {
		config.Logger.Error("Failed to fetch complete group details",
			zap.Error(err),
			zap.String("groupId", createdGroup.ID.String()))
		// Continue with created group even if fetch fails
		completeGroup = createdGroup
	}

	// Find the final approver member for response
	var finalApproverMember *models.ApprovalGroupMember
	for _, member := range completeGroup.Members {
		if member.IsFinalApprover {
			finalApproverMember = &member
			break
		}
	}

	response := fiber.Map{
		"message": "Approval group successfully created with members",
		"data": fiber.Map{
			"group":          completeGroup,
			"final_approver": finalApproverMember,
		},
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

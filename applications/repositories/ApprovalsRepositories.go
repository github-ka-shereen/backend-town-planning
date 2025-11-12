package repositories

import (
	"errors"
	"fmt"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ProcessApplicationApproval handles the approval of an application by a group member
func (r *applicationRepository) ProcessApplicationApproval(
	tx *gorm.DB,
	applicationID string,
	userID uuid.UUID,
	comment *string,
	commentType models.CommentType,
) (*ApprovalResult, error) {
	// Fetch application with group assignment and members
	var application models.Application
	err := tx.
		Preload("ApprovalGroup.Members", "is_active = ?", true).
		Preload("GroupAssignments", "is_active = ?", true).
		Preload("GroupAssignments.Decisions").
		Where("id = ?", applicationID).
		First(&application).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("application not found")
		}
		return nil, err
	}

	// Check if user is a member of the approval group
	var groupMember models.ApprovalGroupMember
	err = tx.
		Where("approval_group_id = ? AND user_id = ? AND is_active = ?",
			application.ApprovalGroup.ID, userID, true).
		First(&groupMember).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not authorized to approve this application")
		}
		return nil, err
	}

	// Check if user can approve
	if !groupMember.CanApprove {
		return nil, errors.New("user does not have permission to approve applications")
	}

	// Check if there's an active group assignment
	if len(application.GroupAssignments) == 0 {
		return nil, errors.New("no active group assignment found for this application")
	}

	assignment := application.GroupAssignments[0]

	// Check if user already made a decision
	var existingDecision models.MemberApprovalDecision
	err = tx.
		Where("assignment_id = ? AND member_id = ?", assignment.ID, groupMember.ID).
		First(&existingDecision).Error

	now := time.Now()
	var decision models.MemberApprovalDecision

	if err == nil {
		// Update existing decision
		decision = existingDecision
		decision.Status = models.DecisionApproved
		decision.DecidedAt = &now
		decision.UpdatedAt = now
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new decision
		decision = models.MemberApprovalDecision{
			ID:                      uuid.New(),
			AssignmentID:            assignment.ID,
			MemberID:                groupMember.ID,
			UserID:                  userID,
			Status:                  models.DecisionApproved,
			DecidedAt:               &now,
			AssignedAs:              groupMember.Role,
			IsFinalApproverDecision: groupMember.IsFinalApprover,
			WasAvailable:            groupMember.AvailabilityStatus == models.AvailabilityAvailable,
		}
	} else {
		return nil, err
	}

	// Save decision
	if err := tx.Save(&decision).Error; err != nil {
		return nil, err
	}

	// Add comment if provided
	if comment != nil && *comment != "" {
		approvalComment := models.Comment{
			ID:            uuid.New(),
			ApplicationID: application.ID,
			DecisionID:    &decision.ID,
			CommentType:   commentType,
			Content:       *comment,
			UserID:        userID,
			CreatedBy:     fmt.Sprintf("%s %s", groupMember.User.FirstName, groupMember.User.LastName),
		}
		if err := tx.Create(&approvalComment).Error; err != nil {
			return nil, err
		}
	}

	// Update assignment statistics
	if err := r.updateAssignmentStatistics(tx, assignment.ID); err != nil {
		return nil, err
	}

	// ========================================
	// AUTO-REJECTION CHECK FOR APPROVALS
	// ========================================

	if !groupMember.IsFinalApprover {
		// Get all regular members (excluding final approver)
		var regularMembers []models.ApprovalGroupMember
		if err := tx.
			Where("approval_group_id = ? AND is_active = ? AND is_final_approver = ?",
				assignment.ApprovalGroupID, true, false).
			Find(&regularMembers).Error; err != nil {
			return nil, err
		}

		// Count how many regular members have made decisions (not pending)
		var decidedCount int64
		var rejectedCount int64

		if err := tx.Model(&models.MemberApprovalDecision{}).
			Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
			Where("member_approval_decisions.assignment_id = ? AND member_approval_decisions.deleted_at IS NULL", assignment.ID).
			Where("approval_group_members.is_final_approver = ? AND approval_group_members.is_active = ?", false, true).
			Where("member_approval_decisions.status != ?", models.DecisionPending).
			Count(&decidedCount).Error; err != nil {
			return nil, err
		}

		if err := tx.Model(&models.MemberApprovalDecision{}).
			Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
			Where("member_approval_decisions.assignment_id = ? AND member_approval_decisions.deleted_at IS NULL", assignment.ID).
			Where("approval_group_members.is_final_approver = ? AND approval_group_members.is_active = ?", false, true).
			Where("member_approval_decisions.status = ?", models.DecisionRejected).
			Count(&rejectedCount).Error; err != nil {
			return nil, err
		}

		regularMemberCount := int64(len(regularMembers))
		allRegularMembersDecided := decidedCount >= regularMemberCount
		hasAnyRejection := rejectedCount > 0

		config.Logger.Info("Auto-rejection check on approval",
			zap.String("applicationID", applicationID),
			zap.Int64("regularMemberCount", regularMemberCount),
			zap.Int64("decidedCount", decidedCount),
			zap.Int64("rejectedCount", rejectedCount),
			zap.Bool("allDecided", allRegularMembersDecided),
			zap.Bool("hasRejection", hasAnyRejection))

		// If all regular members decided AND there's any rejection -> AUTO-REJECT
		if allRegularMembersDecided && hasAnyRejection {
			application.Status = models.RejectedApplication
			assignment.CompletedAt = &now
			assignment.FinalDecisionAt = &now
			assignment.ReadyForFinalApproval = false

			// Get the actual final approver from the group
			var finalApproverMember models.ApprovalGroupMember
			err = tx.
				Where("approval_group_id = ? AND is_final_approver = ? AND is_active = ?",
					assignment.ApprovalGroupID, true, true).
				First(&finalApproverMember).Error

			if err != nil {
				return nil, fmt.Errorf("failed to find final approver for auto-rejection: %w", err)
			}

			// Create final approval record for the auto-rejection
			rejectionReason := "Application auto-rejected due to member rejections"
			finalApproval := models.FinalApproval{
				ID:                    uuid.New(),
				ApplicationID:         application.ID,
				ApproverID:            finalApproverMember.UserID,
				Decision:              models.RejectedApplication,
				DecisionAt:            now,
				Comment:               &rejectionReason,
				OverrodeGroupDecision: false,
				IsSystemAutoDecision:  true,
			}
			if err := tx.Create(&finalApproval).Error; err != nil {
				return nil, err
			}
			assignment.FinalDecisionID = &finalApproval.ID

			if err := tx.Save(&application).Error; err != nil {
				return nil, err
			}
			if err := tx.Save(&assignment).Error; err != nil {
				return nil, err
			}

			config.Logger.Info("Auto-rejected application after approval (other members rejected)",
				zap.String("applicationID", applicationID),
				zap.String("approvingMember", groupMember.User.FirstName+" "+groupMember.User.LastName))

			// Prepare result
			result := &ApprovalResult{
				ApplicationStatus:     application.Status,
				IsFinalApprover:       false,
				ReadyForFinalApproval: false,
				ApprovedCount:         assignment.ApprovedCount,
				TotalMembers:          assignment.TotalMembers,
				UnresolvedIssues:      assignment.IssuesRaised - assignment.IssuesResolved,
			}

			return result, nil
		}

		// If all regular members decided AND no rejections -> Ready for final approval
		if allRegularMembersDecided && !hasAnyRejection {
			assignment.ReadyForFinalApproval = true
			assignment.FinalApproverAssignedAt = &now
			if err := tx.Save(&assignment).Error; err != nil {
				return nil, err
			}

			config.Logger.Info("All regular members approved, ready for final approval",
				zap.String("applicationID", applicationID))
		}
	}

	// Update application status if final approver
	if groupMember.IsFinalApprover {
		isReadyForFinalApproval := r.isAssignmentReadyForFinalApproval(tx, &assignment)

		if isReadyForFinalApproval {
			application.Status = models.ApprovedApplication
			assignment.CompletedAt = &now
			assignment.FinalDecisionAt = &now

			// Get application's final approver
			var finalApproverMember models.ApprovalGroupMember
			err = tx.
				Where("approval_group_id = ? AND is_final_approver = ? AND is_active = ?",
					assignment.ApprovalGroupID, true, true).
				First(&finalApproverMember).Error

			if err != nil {
				return nil, fmt.Errorf("failed to find final approver: %w", err)
			}

			// Check if there's an active final approval (non-deleted)
			var existingActiveFinalApproval models.FinalApproval
			err = tx.Where("application_id = ? AND deleted_at IS NULL", application.ID).
				First(&existingActiveFinalApproval).Error

			if err == nil {
				// Active final approval exists - this shouldn't happen after revocation
				config.Logger.Warn("Active final approval already exists, but proceeding",
					zap.String("applicationID", applicationID),
					zap.String("finalApprovalID", existingActiveFinalApproval.ID.String()))

				// Option 1: Update existing one
				existingActiveFinalApproval.ApproverID = finalApproverMember.UserID
				existingActiveFinalApproval.Decision = models.ApprovedApplication
				existingActiveFinalApproval.DecisionAt = now
				existingActiveFinalApproval.Comment = comment
				existingActiveFinalApproval.UpdatedAt = now

				if err := tx.Save(&existingActiveFinalApproval).Error; err != nil {
					return nil, fmt.Errorf("failed to update final approval: %w", err)
				}
				assignment.FinalDecisionID = &existingActiveFinalApproval.ID

			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				// No active final approval exists - create new one (NORMAL CASE after revocation)
				finalApproval := models.FinalApproval{
					ID:            uuid.New(),
					ApplicationID: application.ID,
					ApproverID:    finalApproverMember.UserID,
					Decision:      models.ApprovedApplication,
					DecisionAt:    now,
					Comment:       comment,
				}
				if err := tx.Create(&finalApproval).Error; err != nil {
					return nil, fmt.Errorf("failed to create final approval: %w", err)
				}

				assignment.FinalDecisionID = &finalApproval.ID

				config.Logger.Info("Created new final approval",
					zap.String("applicationID", applicationID),
					zap.String("finalApprovalID", finalApproval.ID.String()),
					zap.String("approverID", userID.String()))
			} else {
				return nil, err
			}

			if err := tx.Save(&application).Error; err != nil {
				return nil, err
			}
			if err := tx.Save(&assignment).Error; err != nil {
				return nil, err
			}
		} else {
			config.Logger.Warn("Final approver attempted to approve application not ready for final approval",
				zap.String("applicationID", applicationID),
				zap.String("userID", userID.String()))
		}
	}

	// Prepare result
	result := &ApprovalResult{
		ApplicationStatus:     application.Status,
		IsFinalApprover:       groupMember.IsFinalApprover,
		ReadyForFinalApproval: assignment.ReadyForFinalApproval,
		ApprovedCount:         assignment.ApprovedCount,
		TotalMembers:          assignment.TotalMembers,
		UnresolvedIssues:      assignment.IssuesRaised - assignment.IssuesResolved,
	}

	// If final approver just approved, update the ready status
	if groupMember.IsFinalApprover {
		result.ReadyForFinalApproval = r.isAssignmentReadyForFinalApproval(tx, &assignment)
	}

	return result, nil
}

// ProcessApplicationRejection handles the rejection of an application by a group member
func (r *applicationRepository) ProcessApplicationRejection(
	tx *gorm.DB,
	applicationID string,
	userID uuid.UUID,
	reason string,
	comment *string,
	commentType models.CommentType,
) (*RejectionResult, error) {
	// Fetch application with group assignment
	var application models.Application
	err := tx.
		Preload("ApprovalGroup.Members", "is_active = ?", true).
		Preload("GroupAssignments", "is_active = ?", true).
		Preload("FinalApproval").
		Where("id = ?", applicationID).
		First(&application).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("application not found")
		}
		return nil, err
	}

	// Check if user is a member of the approval group
	var groupMember models.ApprovalGroupMember
	err = tx.
		Where("approval_group_id = ? AND user_id = ? AND is_active = ?",
			application.ApprovalGroup.ID, userID, true).
		First(&groupMember).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not authorized to reject this application")
		}
		return nil, err
	}

	// Check if user can reject
	if !groupMember.CanReject {
		return nil, errors.New("user does not have permission to reject applications")
	}

	// Check if there's an active group assignment
	if len(application.GroupAssignments) == 0 {
		return nil, errors.New("no active group assignment found for this application")
	}

	assignment := application.GroupAssignments[0]
	now := time.Now()

	// Check if user already made a decision
	var existingDecision models.MemberApprovalDecision
	err = tx.
		Where("assignment_id = ? AND member_id = ?", assignment.ID, groupMember.ID).
		First(&existingDecision).Error

	var decision models.MemberApprovalDecision

	if err == nil {
		// Update existing decision
		decision = existingDecision
		decision.Status = models.DecisionRejected
		decision.DecidedAt = &now
		decision.UpdatedAt = now
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new decision only if none exists
		decision = models.MemberApprovalDecision{
			ID:                      uuid.New(),
			AssignmentID:            assignment.ID,
			MemberID:                groupMember.ID,
			UserID:                  userID,
			Status:                  models.DecisionRejected,
			DecidedAt:               &now,
			AssignedAs:              groupMember.Role,
			IsFinalApproverDecision: groupMember.IsFinalApprover,
			WasAvailable:            groupMember.AvailabilityStatus == models.AvailabilityAvailable,
		}
	} else {
		return nil, err
	}

	// Save decision (this will update if existing, create if new)
	if err := tx.Save(&decision).Error; err != nil {
		return nil, err
	}

	// Add rejection comment
	rejectionContent := fmt.Sprintf("REJECTION REASON: %s", reason)
	if comment != nil && *comment != "" {
		rejectionContent = fmt.Sprintf("%s\nADDITIONAL COMMENTS: %s", rejectionContent, *comment)
	}

	rejectionComment := models.Comment{
		ID:            uuid.New(),
		ApplicationID: application.ID,
		DecisionID:    &decision.ID,
		CommentType:   commentType,
		Content:       rejectionContent,
		UserID:        userID,
		CreatedBy:     fmt.Sprintf("%s %s", groupMember.User.FirstName, groupMember.User.LastName),
	}
	if err := tx.Create(&rejectionComment).Error; err != nil {
		return nil, err
	}

	// Update assignment statistics
	if err := r.updateAssignmentStatistics(tx, assignment.ID); err != nil {
		return nil, err
	}

	// ========================================
	// AUTO-REJECTION LOGIC FOR REJECTIONS
	// ========================================

	// Get all regular members (excluding final approver)
	var regularMembers []models.ApprovalGroupMember
	if err := tx.
		Where("approval_group_id = ? AND is_active = ? AND is_final_approver = ?",
			assignment.ApprovalGroupID, true, false).
		Find(&regularMembers).Error; err != nil {
		return nil, err
	}

	// Count how many regular members have made decisions (not pending)
	var decidedCount int64
	var rejectedCount int64

	if err := tx.Model(&models.MemberApprovalDecision{}).
		Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
		Where("member_approval_decisions.assignment_id = ? AND member_approval_decisions.deleted_at IS NULL", assignment.ID).
		Where("approval_group_members.is_final_approver = ? AND approval_group_members.is_active = ?", false, true).
		Where("member_approval_decisions.status != ?", models.DecisionPending).
		Count(&decidedCount).Error; err != nil {
		return nil, err
	}

	if err := tx.Model(&models.MemberApprovalDecision{}).
		Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
		Where("member_approval_decisions.assignment_id = ? AND member_approval_decisions.deleted_at IS NULL", assignment.ID).
		Where("approval_group_members.is_final_approver = ? AND approval_group_members.is_active = ?", false, true).
		Where("member_approval_decisions.status = ?", models.DecisionRejected).
		Count(&rejectedCount).Error; err != nil {
		return nil, err
	}

	regularMemberCount := int64(len(regularMembers))
	allRegularMembersDecided := decidedCount >= regularMemberCount
	hasAnyRejection := rejectedCount > 0

	config.Logger.Info("Auto-rejection check on rejection",
		zap.String("applicationID", applicationID),
		zap.Int64("regularMemberCount", regularMemberCount),
		zap.Int64("decidedCount", decidedCount),
		zap.Int64("rejectedCount", rejectedCount),
		zap.Bool("allDecided", allRegularMembersDecided),
		zap.Bool("hasRejection", hasAnyRejection),
		zap.Bool("isFinalApprover", groupMember.IsFinalApprover))

	// PHASE 1: Regular member rejects, but not all members have decided yet
	if !groupMember.IsFinalApprover && !allRegularMembersDecided {
		// Keep application under review - other members still need to review
		application.Status = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = false

		config.Logger.Info("Regular member rejected, waiting for other members",
			zap.String("applicationID", applicationID),
			zap.String("rejectingMember", groupMember.User.FirstName+" "+groupMember.User.LastName))

		// PHASE 2: All regular members decided, check if we should auto-reject
	} else if !groupMember.IsFinalApprover && allRegularMembersDecided && hasAnyRejection {
		// AUTO-REJECT: At least one regular member rejected, no need for final approver
		application.Status = models.RejectedApplication
		assignment.CompletedAt = &now
		assignment.FinalDecisionAt = &now
		assignment.ReadyForFinalApproval = false

		// Get the actual final approver from the group
		var finalApproverMember models.ApprovalGroupMember
		err = tx.
			Where("approval_group_id = ? AND is_final_approver = ? AND is_active = ?",
				assignment.ApprovalGroupID, true, true).
			First(&finalApproverMember).Error

		if err != nil {
			return nil, fmt.Errorf("failed to find final approver for auto-rejection: %w", err)
		}

		// Create final approval record for the auto-rejection using FINAL APPROVER's ID
		finalApproval := models.FinalApproval{
			ID:                    uuid.New(),
			ApplicationID:         application.ID,
			ApproverID:            finalApproverMember.UserID,
			Decision:              models.RejectedApplication,
			DecisionAt:            now,
			Comment:               &rejectionContent,
			OverrodeGroupDecision: false,
			IsSystemAutoDecision:  true,
		}
		if err := tx.Create(&finalApproval).Error; err != nil {
			return nil, err
		}
		assignment.FinalDecisionID = &finalApproval.ID

		config.Logger.Info("Auto-rejected application due to regular member rejection",
			zap.String("applicationID", applicationID),
			zap.String("finalApproverID", finalApproverMember.UserID.String()),
			zap.Int64("rejectedCount", rejectedCount))

		// PHASE 3: All regular members approved, ready for final approver (shouldn't happen in rejection flow)
	} else if !groupMember.IsFinalApprover && allRegularMembersDecided && !hasAnyRejection {
		// All regular members approved - ready for final approval
		application.Status = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = true
		assignment.FinalApproverAssignedAt = &now

		config.Logger.Info("All regular members approved, ready for final approval",
			zap.String("applicationID", applicationID))

		// FINAL APPROVER rejection
	} else if groupMember.IsFinalApprover {
		// Final approver rejection - normal process
		application.Status = models.RejectedApplication
		assignment.CompletedAt = &now
		assignment.FinalDecisionAt = &now

		// Handle final approval record (update existing or create new)
		var existingActiveFinalApproval models.FinalApproval
		err = tx.Where("application_id = ? AND deleted_at IS NULL", application.ID).
			First(&existingActiveFinalApproval).Error

		if err == nil {
			// Update existing final approval
			existingActiveFinalApproval.ApproverID = userID
			existingActiveFinalApproval.Decision = models.RejectedApplication
			existingActiveFinalApproval.DecisionAt = now
			existingActiveFinalApproval.Comment = &rejectionContent
			existingActiveFinalApproval.UpdatedAt = now

			if err := tx.Save(&existingActiveFinalApproval).Error; err != nil {
				return nil, err
			}
			assignment.FinalDecisionID = &existingActiveFinalApproval.ID

			config.Logger.Info("Updated existing final approval for rejection",
				zap.String("applicationID", applicationID),
				zap.String("finalApprovalID", existingActiveFinalApproval.ID.String()))
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new final approval
			finalApproval := models.FinalApproval{
				ID:            uuid.New(),
				ApplicationID: application.ID,
				ApproverID:    userID,
				Decision:      models.RejectedApplication,
				DecisionAt:    now,
				Comment:       &rejectionContent,
			}
			if err := tx.Create(&finalApproval).Error; err != nil {
				return nil, err
			}
			assignment.FinalDecisionID = &finalApproval.ID

			config.Logger.Info("Created new final approval for rejection",
				zap.String("applicationID", applicationID),
				zap.String("finalApprovalID", finalApproval.ID.String()))
		} else {
			return nil, err
		}
	}

	// Save changes
	if err := tx.Save(&application).Error; err != nil {
		return nil, err
	}
	if err := tx.Save(&assignment).Error; err != nil {
		return nil, err
	}

	result := &RejectionResult{
		ApplicationStatus: application.Status,
		IsFinalApprover:   groupMember.IsFinalApprover,
	}

	return result, nil
}

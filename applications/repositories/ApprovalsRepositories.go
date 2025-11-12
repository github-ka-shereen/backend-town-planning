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

	// Check if ready for final approval (for regular members)
	if !groupMember.IsFinalApprover {
		readyForFinalApproval := assignment.AllRegularMembersApproved() &&
			assignment.IssuesRaised == assignment.IssuesResolved

		if readyForFinalApproval {
			assignment.ReadyForFinalApproval = true
			now := time.Now()
			assignment.FinalApproverAssignedAt = &now
			if err := tx.Save(&assignment).Error; err != nil {
				return nil, err
			}
		}
	}

	// Update application status if final approver
	if groupMember.IsFinalApprover {
		isReadyForFinalApproval := assignment.AllRegularMembersApproved() &&
			assignment.IssuesRaised == assignment.IssuesResolved

		if isReadyForFinalApproval {
			application.Status = models.ApprovedApplication
			assignment.CompletedAt = &now
			assignment.FinalDecisionAt = &now

			// Check if there's an active final approval (non-deleted)
			var existingActiveFinalApproval models.FinalApproval
			err = tx.Where("application_id = ? AND deleted_at IS NULL", application.ID).
				First(&existingActiveFinalApproval).Error

			if err == nil {
				// Active final approval exists - this shouldn't happen, but handle it
				return nil, errors.New("active final approval already exists for this application")
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				// No active final approval exists - create new one
				finalApproval := models.FinalApproval{
					ID:            uuid.New(),
					ApplicationID: application.ID,
					ApproverID:    userID,
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
		result.ReadyForFinalApproval = assignment.AllRegularMembersApproved() &&
			assignment.IssuesRaised == assignment.IssuesResolved
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

	// Create rejection decision
	decision := models.MemberApprovalDecision{
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

	if err := tx.Create(&decision).Error; err != nil {
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
	// SMART TWO-PHASE REJECTION LOGIC
	// ========================================

	// Get updated counts after this rejection
	var updatedAssignment models.ApplicationGroupAssignment
	if err := tx.Where("id = ?", assignment.ID).First(&updatedAssignment).Error; err != nil {
		return nil, err
	}

	// Get all regular members count (excluding final approver)
	var regularMemberCount int64
	if err := tx.Model(&models.ApprovalGroupMember{}).
		Where("approval_group_id = ? AND is_active = ? AND is_final_approver = ?",
			assignment.ApprovalGroupID, true, false).
		Count(&regularMemberCount).Error; err != nil {
		return nil, err
	}

	// Check if ALL regular members have decided (approved or rejected)
	allRegularMembersDecided := (updatedAssignment.ApprovedCount + updatedAssignment.RejectedCount) >= int(regularMemberCount)
	hasAnyRejection := updatedAssignment.RejectedCount > 0

	// PHASE 1: Regular member rejects, but not all members have decided yet
	if !groupMember.IsFinalApprover && !allRegularMembersDecided {
		// Keep application under review - other departments still need to review
		application.Status = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = false

		config.Logger.Info("Regular member rejected, waiting for other departments",
			zap.String("applicationID", applicationID),
			zap.String("rejectingMember", groupMember.User.FirstName+" "+groupMember.User.LastName),
			zap.Int("approvedCount", updatedAssignment.ApprovedCount),
			zap.Int("rejectedCount", updatedAssignment.RejectedCount),
			zap.Int64("totalRegularMembers", regularMemberCount))

		// PHASE 2: All regular members decided, check if we should auto-reject
	} else if !groupMember.IsFinalApprover && allRegularMembersDecided && hasAnyRejection {
		// AUTO-REJECT: At least one regular member rejected, no need for final approver
		application.Status = models.RejectedApplication
		assignment.CompletedAt = &now
		assignment.FinalDecisionAt = &now
		assignment.ReadyForFinalApproval = false

		// Create final approval record for the auto-rejection
		finalApproval := models.FinalApproval{
			ID:                    uuid.New(),
			ApplicationID:         application.ID,
			ApproverID:            userID, // The last rejecting member
			Decision:              models.RejectedApplication,
			DecisionAt:            now,
			Comment:               &rejectionContent,
			OverrodeGroupDecision: false,
		}
		if err := tx.Create(&finalApproval).Error; err != nil {
			return nil, err
		}
		assignment.FinalDecisionID = &finalApproval.ID

		config.Logger.Info("Auto-rejected application due to regular member rejection",
			zap.String("applicationID", applicationID),
			zap.Int("rejectedCount", updatedAssignment.RejectedCount))

		// PHASE 2: All regular members approved, ready for final approver
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

// updateAssignmentStatistics updates the statistics for a group assignment
func (r *applicationRepository) updateAssignmentStatistics(tx *gorm.DB, assignmentID uuid.UUID) error {
	var assignment models.ApplicationGroupAssignment

	if err := tx.Where("id = ?", assignmentID).First(&assignment).Error; err != nil {
		return fmt.Errorf("failed to find assignment: %w", err)
	}

	// Count decisions by status with better error handling
	var stats struct {
		ApprovedCount int64
		RejectedCount int64
		PendingCount  int64
		TotalMembers  int64
	}

	// Get total active members
	if err := tx.Model(&models.ApprovalGroupMember{}).
		Where("approval_group_id = ? AND is_active = ?", assignment.ApprovalGroupID, true).
		Count(&stats.TotalMembers).Error; err != nil {
		return fmt.Errorf("failed to count members: %w", err)
	}

	// Count ONLY NON-DELETED decisions (soft delete aware)
	if err := tx.Model(&models.MemberApprovalDecision{}).
		Where("assignment_id = ? AND deleted_at IS NULL", assignmentID).
		Select("COUNT(CASE WHEN status = 'APPROVED' THEN 1 END) as approved_count, " +
			"COUNT(CASE WHEN status = 'REJECTED' THEN 1 END) as rejected_count, " +
			"COUNT(CASE WHEN status = 'PENDING' THEN 1 END) as pending_count").
		Scan(&stats).Error; err != nil {
		return fmt.Errorf("failed to count decisions: %w", err)
	}

	// Count resolved issues
	var resolvedIssues int64
	if err := tx.Model(&models.ApplicationIssue{}).
		Where("assignment_id = ? AND is_resolved = ?", assignmentID, true).
		Count(&resolvedIssues).Error; err != nil {
		return fmt.Errorf("failed to count resolved issues: %w", err)
	}

	// Update assignment
	assignment.ApprovedCount = int(stats.ApprovedCount)
	assignment.RejectedCount = int(stats.RejectedCount)
	assignment.PendingCount = int(stats.PendingCount)
	assignment.TotalMembers = int(stats.TotalMembers)
	assignment.IssuesResolved = int(resolvedIssues)

	if err := tx.Save(&assignment).Error; err != nil {
		return fmt.Errorf("failed to save assignment: %w", err)
	}

	return nil
}

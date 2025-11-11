package repositories

import (
	"errors"
	"fmt"
	"time"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
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

	// Check if user already made a decision - FIXED VERSION
	var existingDecision models.MemberApprovalDecision
	err = tx.
		Where("assignment_id = ? AND member_id = ?", assignment.ID, groupMember.ID). // Use member_id
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
		// This shouldn't happen if initial decisions were created, but handle it
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

	// Check if ready for final approval
	readyForFinalApproval := false
	if !groupMember.IsFinalApprover {
		readyForFinalApproval = assignment.AllRegularMembersApproved() &&
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
	if groupMember.IsFinalApprover && readyForFinalApproval {
		application.Status = models.ApprovedApplication
		assignment.CompletedAt = &now
		assignment.FinalDecisionAt = &now

		// Create final approval record
		finalApproval := models.FinalApproval{
			ID:            uuid.New(),
			ApplicationID: application.ID,
			ApproverID:    userID,
			Decision:      models.ApprovedApplication,
			DecisionAt:    now,
			Comment:       comment,
		}
		if err := tx.Create(&finalApproval).Error; err != nil {
			return nil, err
		}

		if err := tx.Save(&application).Error; err != nil {
			return nil, err
		}
		if err := tx.Save(&assignment).Error; err != nil {
			return nil, err
		}
	}

	// Prepare result
	result := &ApprovalResult{
		ApplicationStatus:     application.Status,
		IsFinalApprover:       groupMember.IsFinalApprover,
		ReadyForFinalApproval: readyForFinalApproval,
		ApprovedCount:         assignment.ApprovedCount,
		TotalMembers:          assignment.TotalMembers,
		UnresolvedIssues:      assignment.IssuesRaised - assignment.IssuesResolved,
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

	// Update application status
	application.Status = models.RejectedApplication
	assignment.CompletedAt = &now

	if groupMember.IsFinalApprover {
		assignment.FinalDecisionAt = &now
		// Create final approval record for rejection
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
	}

	if err := tx.Save(&application).Error; err != nil {
		return nil, err
	}
	if err := tx.Save(&assignment).Error; err != nil {
		return nil, err
	}

	// Update assignment statistics
	if err := r.updateAssignmentStatistics(tx, assignment.ID); err != nil {
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

	// Count decisions by status
	var stats struct {
		ApprovedCount int64
		RejectedCount int64
		PendingCount  int64
		TotalMembers  int64
	}

	// Get total active members in the group
	if err := tx.Model(&models.ApprovalGroupMember{}).
		Where("approval_group_id = (SELECT approval_group_id FROM application_group_assignments WHERE id = ?)", assignmentID).
		Where("is_active = ?", true).
		Count(&stats.TotalMembers).Error; err != nil {
		return err
	}

	// Count decisions by status
	if err := tx.Model(&models.MemberApprovalDecision{}).
		Where("assignment_id = ?", assignmentID).
		Select("COUNT(CASE WHEN status = 'APPROVED' THEN 1 END) as approved_count, " +
			"COUNT(CASE WHEN status = 'REJECTED' THEN 1 END) as rejected_count, " +
			"COUNT(CASE WHEN status = 'PENDING' THEN 1 END) as pending_count").
		Scan(&stats).Error; err != nil {
		return err
	}

	// Count resolved issues
	var resolvedIssues int64
	if err := tx.Model(&models.ApplicationIssue{}).
		Where("assignment_id = ? AND is_resolved = ?", assignmentID, true).
		Count(&resolvedIssues).Error; err != nil {
		return err
	}

	// Update assignment
	if err := tx.Where("id = ?", assignmentID).First(&assignment).Error; err != nil {
		return err
	}

	assignment.ApprovedCount = int(stats.ApprovedCount)
	assignment.RejectedCount = int(stats.RejectedCount)
	assignment.PendingCount = int(stats.PendingCount)
	assignment.TotalMembers = int(stats.TotalMembers)
	assignment.IssuesResolved = int(resolvedIssues)

	return tx.Save(&assignment).Error
}

package repositories

import (
	"errors"
	"fmt"
	"time"

	"town-planning-backend/applications/requests"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProcessDecisionRevocation handles revoking a user's decision by soft deleting it and resetting everything
func (r *applicationRepository) ProcessDecisionRevocation(
	tx *gorm.DB,
	applicationID string,
	userID uuid.UUID,
	reason string,
) (*requests.RevokeDecisionResponse, error) {
	// Fetch application with group assignment and decisions
	var application models.Application
	err := tx.
		Preload("ApprovalGroup.Members", "is_active = ?", true).
		Preload("GroupAssignments", "is_active = ?", true).
		Preload("GroupAssignments.Decisions").
		Preload("GroupAssignments.Decisions.Comments").
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
			return nil, errors.New("user not authorized to revoke decision")
		}
		return nil, err
	}

	// Check if there's an active group assignment
	if len(application.GroupAssignments) == 0 {
		return nil, errors.New("no active group assignment found for this application")
	}

	assignment := application.GroupAssignments[0]

	// Find the user's decision
	var userDecision models.MemberApprovalDecision
	err = tx.
		Where("assignment_id = ? AND user_id = ?", assignment.ID, userID).
		Preload("Comments").
		First(&userDecision).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("no decision found to revoke")
		}
		return nil, err
	}

	// Store the original decision status BEFORE any changes
	originalDecisionStatus := userDecision.Status

	// Check if user can revoke based on application status and user role
	canRevoke, err := r.canUserRevokeDecision(&application, &groupMember, &userDecision)
	if err != nil {
		return nil, err
	}

	if !canRevoke {
		return nil, errors.New("cannot revoke decision in current application state")
	}

	now := time.Now()

	// Create revocation record for audit trail
	revocation := models.DecisionRevocation{
		ID:             uuid.New(),
		DecisionID:     userDecision.ID,
		RevokedBy:      userID,
		Reason:         reason,
		RevokedAt:      now,
		PreviousStatus: originalDecisionStatus, // Use the stored original status
	}

	if err := tx.Create(&revocation).Error; err != nil {
		return nil, err
	}

	// SOFT DELETE all comments associated with this decision first
	if len(userDecision.Comments) > 0 {
		// Soft delete all comments linked to this decision
		if err := tx.Model(&models.Comment{}).
			Where("decision_id = ?", userDecision.ID).
			Update("deleted_at", now).Error; err != nil {
			return nil, fmt.Errorf("failed to soft delete decision comments: %w", err)
		}
	}

	// Reset the decision to pending state
	userDecision.Status = models.DecisionPending
	userDecision.DecidedAt = nil

	if err := tx.Model(&userDecision).Select("status", "decided_at").Updates(userDecision).Error; err != nil {
		return nil, fmt.Errorf("failed to reset decision status: %w", err)
	}

	// Update application status based on the revocation
	result := &requests.RevocationResult{
		WasFinalApprover: groupMember.IsFinalApprover,
		PreviousStatus:   application.Status,
	}

	// Reset application and assignment based on who revoked and what was revoked
	// Use originalDecisionStatus instead of userDecision.Status (which is now PENDING)
	switch {
	case groupMember.IsFinalApprover && application.Status == models.ApprovedApplication:
		// Final approver revoking final approval - revert to ready for final approval
		application.Status = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = true
		assignment.CompletedAt = nil
		assignment.FinalDecisionAt = nil
		result.ReadyForFinalApproval = true
		result.Message = "Final approval revoked, application returned for review"

	case groupMember.IsFinalApprover && application.Status == models.RejectedApplication:
		// Final approver revoking rejection - revert to ready for final approval
		application.Status = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = true
		assignment.CompletedAt = nil
		assignment.FinalDecisionAt = nil
		result.ReadyForFinalApproval = true
		result.Message = "Rejection revoked, application returned for review"

	case !groupMember.IsFinalApprover && originalDecisionStatus == models.DecisionApproved:
		// Regular member revoking approval
		application.Status = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = false
		result.ReadyForFinalApproval = false
		result.Message = "Approval revoked"

	case !groupMember.IsFinalApprover && originalDecisionStatus == models.DecisionRejected:
		// Regular member revoking rejection
		application.Status = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = false
		result.ReadyForFinalApproval = false
		result.Message = "Rejection revoked"

	default:
		return nil, errors.New("cannot revoke decision in current state")
	}

	// Save application and assignment changes
	if err := tx.Save(&application).Error; err != nil {
		return nil, err
	}

	if err := tx.Save(&assignment).Error; err != nil {
		return nil, err
	}

	// Update assignment statistics (this will now exclude soft-deleted decisions)
	if err := r.updateAssignmentStatistics(tx, assignment.ID); err != nil {
		return nil, err
	}

	result.NewStatus = application.Status

	return &requests.RevokeDecisionResponse{
		Success:               true,
		Message:               result.Message,
		NewStatus:             result.NewStatus,
		ReadyForFinalApproval: result.ReadyForFinalApproval,
		WasFinalApprover:      result.WasFinalApprover,
		PreviousStatus:        string(result.PreviousStatus),
	}, nil
}

// canUserRevokeDecision checks if a user can revoke their decision
func (r *applicationRepository) canUserRevokeDecision(
	application *models.Application,
	groupMember *models.ApprovalGroupMember,
	decision *models.MemberApprovalDecision,
) (bool, error) {

	// Cannot revoke if no decision was made
	if decision.Status == models.DecisionPending {
		return false, errors.New("no decision to revoke")
	}

	// If application has final decision (approved/rejected), only final approver can revoke their own decision
	if application.Status == models.ApprovedApplication || application.Status == models.RejectedApplication {
		if !groupMember.IsFinalApprover {
			return false, errors.New("only final approver can revoke after final decision")
		}
		if !decision.IsFinalApproverDecision {
			return false, errors.New("cannot revoke regular member decisions after final decision")
		}
		return true, nil
	}

	// For all other statuses (under review, pending, etc.), allow revocation
	return true, nil
}
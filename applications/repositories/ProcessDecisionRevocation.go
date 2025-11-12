package repositories

import (
	"errors"
	"fmt"
	"time"
	"town-planning-backend/applications/requests"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ProcessDecisionRevocation handles revoking a user's decision on an application
func (r *applicationRepository) ProcessDecisionRevocation(
	tx *gorm.DB,
	applicationID string,
	userID uuid.UUID,
	reason string,
) (*requests.RevocationResult, error) {
	// Step 1: Fetch application with all necessary data
	var application models.Application
	err := tx.
		Preload("ApprovalGroup.Members", "is_active = ?", true).
		Preload("GroupAssignments", "is_active = ?", true).
		Preload("GroupAssignments.Decisions").
		Preload("FinalApproval").
		Where("id = ?", applicationID).
		First(&application).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("application not found")
		}
		return nil, err
	}

	// Store previous status for response
	previousStatus := application.Status

	// Step 2: Verify user is a group member
	var groupMember models.ApprovalGroupMember
	err = tx.
		Preload("User").
		Where("approval_group_id = ? AND user_id = ? AND is_active = ?",
			application.ApprovalGroup.ID, userID, true).
		First(&groupMember).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not authorized to revoke decision")
		}
		return nil, err
	}

	// Step 3: Check if there's an active assignment
	if len(application.GroupAssignments) == 0 {
		return nil, errors.New("no active group assignment found")
	}

	assignment := application.GroupAssignments[0]

	// Step 4: Find the user's decision
	var decision models.MemberApprovalDecision
	err = tx.
		Where("assignment_id = ? AND member_id = ? AND deleted_at IS NULL",
			assignment.ID, groupMember.ID).
		First(&decision).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("no decision found to revoke")
		}
		return nil, err
	}

	// Step 5: Prevent revoking if decision is already revoked
	if decision.Status == models.DecisionRevoked {
		return nil, errors.New("decision is already revoked")
	}

	// Store the previous status before revoking
	previousDecisionStatus := decision.Status
	now := time.Now()

	config.Logger.Info("Starting decision revocation",
		zap.String("applicationID", applicationID),
		zap.String("userID", userID.String()),
		zap.Bool("isFinalApprover", groupMember.IsFinalApprover),
		zap.String("previousDecisionStatus", string(previousDecisionStatus)),
		zap.String("applicationStatus", string(application.Status)))

	// STEP 6: REMOVE FINAL APPROVAL FOR ANY REVOCATION (NEW LOGIC)
	if application.FinalApproval != nil {
		config.Logger.Info("Removing final approval record due to revocation",
			zap.String("applicationID", application.ID.String()),
			zap.String("finalApprovalID", application.FinalApproval.ID.String()),
			zap.Bool("wasSystemAutoDecision", application.FinalApproval.IsSystemAutoDecision),
			zap.String("revokedByUserType", map[bool]string{true: "FINAL_APPROVER", false: "REGULAR_MEMBER"}[groupMember.IsFinalApprover]))

		if err := tx.Delete(&application.FinalApproval).Error; err != nil {
			return nil, fmt.Errorf("failed to delete final approval: %w", err)
		}

		// Reset assignment fields related to final approval
		assignment.CompletedAt = nil
		assignment.FinalDecisionAt = nil
		assignment.FinalDecisionID = nil
	}

	// Step 7: Handle based on user type
	if groupMember.IsFinalApprover {
		return r.handleFinalApproverRevocation(
			tx,
			&application,
			&assignment,
			&decision,
			&groupMember,
			previousStatus,
			previousDecisionStatus,
			reason,
			now,
		)
	}

	return r.handleRegularMemberRevocation(
		tx,
		&application,
		&assignment,
		&decision,
		&groupMember,
		previousStatus,
		previousDecisionStatus,
		reason,
		now,
	)
}

// handleFinalApproverRevocation handles revocation by the final approver
func (r *applicationRepository) handleFinalApproverRevocation(
	tx *gorm.DB,
	application *models.Application,
	assignment *models.ApplicationGroupAssignment,
	decision *models.MemberApprovalDecision,
	groupMember *models.ApprovalGroupMember,
	previousStatus models.ApplicationStatus,
	previousDecisionStatus models.MemberDecisionStatus,
	reason string,
	now time.Time,
) (*requests.RevocationResult, error) {

	config.Logger.Info("Processing final approver revocation",
		zap.String("applicationID", application.ID.String()),
		zap.String("previousStatus", string(previousStatus)),
		zap.String("previousDecisionStatus", string(previousDecisionStatus)))

	// Step 1: Revoke the decision
	decision.Status = models.DecisionRevoked
	decision.WasRevoked = true
	decision.RevokedBy = &groupMember.User.Email
	decision.RevokedAt = &now
	decision.RevokedReason = &reason
	decision.UpdatedAt = now

	if err := tx.Save(decision).Error; err != nil {
		return nil, fmt.Errorf("failed to revoke decision: %w", err)
	}

	// Step 2: Create revocation record
	revocation := models.DecisionRevocation{
		ID:             uuid.New(),
		DecisionID:     decision.ID,
		RevokedBy:      groupMember.UserID,
		Reason:         reason,
		RevokedAt:      now,
		PreviousStatus: previousDecisionStatus,
	}
	if err := tx.Create(&revocation).Error; err != nil {
		return nil, fmt.Errorf("failed to create revocation record: %w", err)
	}

	// Step 3: Check current state of regular members to determine readiness
	regularMembers, err := r.getRegularMembers(tx, assignment.ApprovalGroupID)
	if err != nil {
		return nil, err
	}

	// Count current decisions (excluding revoked ones)
	decidedCount, rejectedCount := r.countActiveRegularDecisions(tx, assignment.ID, regularMembers)
	regularMemberCount := int64(len(regularMembers))
	allRegularApproved := decidedCount == regularMemberCount && rejectedCount == 0
	allRegularDecided := decidedCount == regularMemberCount

	config.Logger.Info("Final approver revocation - checking regular member state",
		zap.String("applicationID", application.ID.String()),
		zap.Int("regularMemberCount", len(regularMembers)),
		zap.Int64("decidedCount", decidedCount),
		zap.Int64("rejectedCount", rejectedCount),
		zap.Bool("allRegularApproved", allRegularApproved),
		zap.Bool("allRegularDecided", allRegularDecided))

	// Step 4: SIMPLE LOGIC - Just reset to appropriate review state
	var newStatus models.ApplicationStatus

	if allRegularApproved {
		// All regular members approved, ready for final approval again
		newStatus = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = true
		assignment.FinalApproverAssignedAt = &now
	} else if allRegularDecided && rejectedCount > 0 {
		// All regular members decided but there are rejections
		// Return to review state - let users decide what to do next
		newStatus = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = false
		config.Logger.Info("All regular members decided with rejections - returning to review state",
			zap.String("applicationID", application.ID.String()))
	} else {
		// Not all decided yet or mixed state
		newStatus = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = false
		assignment.FinalApproverAssignedAt = nil
	}

	// Step 5: Update application status and reset dates
	application.Status = newStatus
	application.FinalApprovalDate = nil
	application.RejectionDate = nil
	application.ReviewCompletedAt = nil

	// Step 6: Save changes
	if err := tx.Save(application).Error; err != nil {
		return nil, fmt.Errorf("failed to update application: %w", err)
	}
	if err := tx.Save(assignment).Error; err != nil {
		return nil, fmt.Errorf("failed to update assignment: %w", err)
	}

	// Step 7: Update statistics
	if err := r.updateAssignmentStatistics(tx, assignment.ID); err != nil {
		return nil, err
	}

	config.Logger.Info("Final approver revocation completed",
		zap.String("applicationID", application.ID.String()),
		zap.String("previousStatus", string(previousStatus)),
		zap.String("newStatus", string(newStatus)),
		zap.Bool("readyForFinalApproval", assignment.ReadyForFinalApproval))

	return &requests.RevocationResult{
		NewStatus:             newStatus,
		PreviousStatus:        previousStatus,
		WasFinalApprover:      true,
		ReadyForFinalApproval: assignment.ReadyForFinalApproval,
		Message:               "Final approval revoked successfully - application returned to review state",
	}, nil
}

// handleRegularMemberRevocation handles revocation by a regular member
func (r *applicationRepository) handleRegularMemberRevocation(
	tx *gorm.DB,
	application *models.Application,
	assignment *models.ApplicationGroupAssignment,
	decision *models.MemberApprovalDecision,
	groupMember *models.ApprovalGroupMember,
	previousStatus models.ApplicationStatus,
	previousDecisionStatus models.MemberDecisionStatus,
	reason string,
	now time.Time,
) (*requests.RevocationResult, error) {

	config.Logger.Info("Processing regular member revocation",
		zap.String("applicationID", application.ID.String()),
		zap.String("previousDecisionStatus", string(previousDecisionStatus)),
		zap.String("previousAppStatus", string(previousStatus)))

	// Step 1: Revoke the decision
	decision.Status = models.DecisionRevoked
	decision.WasRevoked = true
	decision.RevokedBy = &groupMember.User.Email
	decision.RevokedAt = &now
	decision.RevokedReason = &reason
	decision.UpdatedAt = now

	if err := tx.Save(decision).Error; err != nil {
		return nil, fmt.Errorf("failed to revoke decision: %w", err)
	}

	// Step 2: Create revocation record
	revocation := models.DecisionRevocation{
		ID:             uuid.New(),
		DecisionID:     decision.ID,
		RevokedBy:      groupMember.UserID,
		Reason:         reason,
		RevokedAt:      now,
		PreviousStatus: previousDecisionStatus,
	}
	if err := tx.Create(&revocation).Error; err != nil {
		return nil, fmt.Errorf("failed to create revocation record: %w", err)
	}

	// Step 3: Determine new application status - SIMPLE RESET LOGIC
	regularMembers, err := r.getRegularMembers(tx, assignment.ApprovalGroupID)
	if err != nil {
		return nil, err
	}

	// Recount decisions (excluding the revoked one)
	decidedCount, rejectedCount := r.countActiveRegularDecisions(tx, assignment.ID, regularMembers)
	regularMemberCount := int64(len(regularMembers))
	allRegularMembersDecided := decidedCount >= regularMemberCount
	allRegularApproved := decidedCount == regularMemberCount && rejectedCount == 0

	config.Logger.Info("Rechecking application state after revocation",
		zap.String("applicationID", application.ID.String()),
		zap.Int64("regularMemberCount", regularMemberCount),
		zap.Int64("decidedCount", decidedCount),
		zap.Int64("rejectedCount", rejectedCount),
		zap.Bool("allDecided", allRegularMembersDecided),
		zap.Bool("allApproved", allRegularApproved),
		zap.String("revokedDecisionType", string(previousDecisionStatus)))

	// SIMPLE LOGIC: Always return to UnderReview and let users decide next actions
	var newStatus models.ApplicationStatus

	if allRegularApproved {
		// All regular members approved - ready for final approval
		newStatus = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = true
		assignment.FinalApproverAssignedAt = &now
		config.Logger.Info("All regular members approved after revocation - ready for final approval",
			zap.String("applicationID", application.ID.String()))
	} else {
		// Not all approved or has rejections - continue review
		newStatus = models.UnderReviewApplication
		assignment.ReadyForFinalApproval = false
		assignment.FinalApproverAssignedAt = nil
		config.Logger.Info("Application returned to review state after revocation",
			zap.String("applicationID", application.ID.String()),
			zap.Int64("pendingCount", regularMemberCount-decidedCount))
	}

	// Step 4: Update application - reset to review state
	application.Status = newStatus
	application.FinalApprovalDate = nil
	application.RejectionDate = nil
	application.ReviewCompletedAt = nil

	// Step 5: Save all changes
	if err := tx.Save(application).Error; err != nil {
		return nil, fmt.Errorf("failed to update application: %w", err)
	}
	if err := tx.Save(assignment).Error; err != nil {
		return nil, fmt.Errorf("failed to update assignment: %w", err)
	}

	// Step 6: Update statistics
	if err := r.updateAssignmentStatistics(tx, assignment.ID); err != nil {
		return nil, err
	}

	config.Logger.Info("Regular member revocation completed",
		zap.String("applicationID", application.ID.String()),
		zap.String("previousStatus", string(previousStatus)),
		zap.String("newStatus", string(newStatus)),
		zap.Bool("readyForFinalApproval", assignment.ReadyForFinalApproval))

	return &requests.RevocationResult{
		NewStatus:             newStatus,
		PreviousStatus:        previousStatus,
		WasFinalApprover:      false,
		ReadyForFinalApproval: assignment.ReadyForFinalApproval,
		Message:               "Decision revoked successfully - application returned to review state",
	}, nil
}

// Helper functions

// getRegularMembers gets all active regular (non-final-approver) members
func (r *applicationRepository) getRegularMembers(
	tx *gorm.DB,
	groupID uuid.UUID,
) ([]models.ApprovalGroupMember, error) {
	var members []models.ApprovalGroupMember
	err := tx.
		Where("approval_group_id = ? AND is_active = ? AND is_final_approver = ?",
			groupID, true, false).
		Find(&members).Error
	return members, err
}

// getFinalApprover gets the active final approver member
func (r *applicationRepository) getFinalApprover(
	tx *gorm.DB,
	groupID uuid.UUID,
) (*models.ApprovalGroupMember, error) {
	var member models.ApprovalGroupMember
	err := tx.
		Where("approval_group_id = ? AND is_active = ? AND is_final_approver = ?",
			groupID, true, true).
		First(&member).Error
	return &member, err
}

// countActiveRegularDecisions counts non-revoked decisions by regular members
func (r *applicationRepository) countActiveRegularDecisions(
	tx *gorm.DB,
	assignmentID uuid.UUID,
	regularMembers []models.ApprovalGroupMember,
) (decidedCount int64, rejectedCount int64) {
	// Count decisions that are approved or rejected (excluding pending and revoked)
	if err := tx.Model(&models.MemberApprovalDecision{}).
		Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
		Where("member_approval_decisions.assignment_id = ?", assignmentID).
		Where("member_approval_decisions.deleted_at IS NULL").
		Where("member_approval_decisions.status != ?", models.DecisionRevoked). // Exclude revoked decisions
		Where("approval_group_members.is_final_approver = ? AND approval_group_members.is_active = ?", false, true).
		Where("member_approval_decisions.status IN (?)", []models.MemberDecisionStatus{
			models.DecisionApproved,
			models.DecisionRejected,
		}).
		Count(&decidedCount).Error; err != nil {
		config.Logger.Error("Failed to count decided regular members", zap.Error(err))
		return 0, 0
	}

	// Count rejections (excluding revoked)
	if err := tx.Model(&models.MemberApprovalDecision{}).
		Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
		Where("member_approval_decisions.assignment_id = ?", assignmentID).
		Where("member_approval_decisions.deleted_at IS NULL").
		Where("member_approval_decisions.status != ?", models.DecisionRevoked). // Exclude revoked decisions
		Where("approval_group_members.is_final_approver = ? AND approval_group_members.is_active = ?", false, true).
		Where("member_approval_decisions.status = ?", models.DecisionRejected).
		Count(&rejectedCount).Error; err != nil {
		config.Logger.Error("Failed to count rejected regular members", zap.Error(err))
		return decidedCount, 0
	}

	return decidedCount, rejectedCount
}

// checkRegularMemberDecisions checks if all regular members approved and if there are rejections
func (r *applicationRepository) checkRegularMemberDecisions(
	assignment *models.ApplicationGroupAssignment,
	regularMembers []models.ApprovalGroupMember,
) (allApproved bool, hasRejections bool) {
	approvedCount := 0
	rejectedCount := 0

	for _, member := range regularMembers {
		for _, decision := range assignment.Decisions {
			if decision.MemberID == member.ID &&
				decision.Status != models.DecisionRevoked &&
				decision.DeletedAt.Time.IsZero() {
				if decision.Status == models.DecisionApproved {
					approvedCount++
				} else if decision.Status == models.DecisionRejected {
					rejectedCount++
				}
				break
			}
		}
	}

	allApproved = approvedCount == len(regularMembers)
	hasRejections = rejectedCount > 0
	return
}

// updateAssignmentStatistics updates the assignment counts
func (r *applicationRepository) updateAssignmentStatistics(tx *gorm.DB, assignmentID uuid.UUID) error {
	// Update the counts based on current decisions
	var stats struct {
		ApprovedCount int64
		RejectedCount int64
		PendingCount  int64
	}

	// Get regular members count
	var regularMemberCount int64
	if err := tx.Model(&models.ApprovalGroupMember{}).
		Where("approval_group_id = (SELECT approval_group_id FROM application_group_assignments WHERE id = ?)", assignmentID).
		Where("is_active = ? AND is_final_approver = ?", true, false).
		Count(&regularMemberCount).Error; err != nil {
		return err
	}

	// Count approved decisions (excluding revoked)
	if err := tx.Model(&models.MemberApprovalDecision{}).
		Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
		Where("member_approval_decisions.assignment_id = ?", assignmentID).
		Where("member_approval_decisions.deleted_at IS NULL").
		Where("member_approval_decisions.status = ?", models.DecisionApproved).
		Where("member_approval_decisions.status != ?", models.DecisionRevoked).
		Where("approval_group_members.is_final_approver = ?", false).
		Count(&stats.ApprovedCount).Error; err != nil {
		return err
	}

	// Count rejected decisions (excluding revoked)
	if err := tx.Model(&models.MemberApprovalDecision{}).
		Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
		Where("member_approval_decisions.assignment_id = ?", assignmentID).
		Where("member_approval_decisions.deleted_at IS NULL").
		Where("member_approval_decisions.status = ?", models.DecisionRejected).
		Where("member_approval_decisions.status != ?", models.DecisionRevoked).
		Where("approval_group_members.is_final_approver = ?", false).
		Count(&stats.RejectedCount).Error; err != nil {
		return err
	}

	// Calculate pending count
	stats.PendingCount = regularMemberCount - stats.ApprovedCount - stats.RejectedCount
	if stats.PendingCount < 0 {
		stats.PendingCount = 0
	}

	// Update the assignment
	if err := tx.Model(&models.ApplicationGroupAssignment{}).
		Where("id = ?", assignmentID).
		Updates(map[string]interface{}{
			"approved_count": stats.ApprovedCount,
			"rejected_count": stats.RejectedCount,
			"pending_count":  stats.PendingCount,
			"total_members":  regularMemberCount,
		}).Error; err != nil {
		return err
	}

	return nil
}

// isAssignmentReadyForFinalApproval checks if an assignment is ready for final approval
// Returns true only if:
// 1. All regular members have approved (no pending, no rejections, no revoked)
// 2. All issues are resolved
// 3. Application is still in review state
func (r *applicationRepository) isAssignmentReadyForFinalApproval(
	tx *gorm.DB,
	assignment *models.ApplicationGroupAssignment,
) bool {
	// Check if all issues are resolved
	if assignment.IssuesRaised != assignment.IssuesResolved {
		return false
	}

	// Get all active regular members
	regularMembers, err := r.getRegularMembers(tx, assignment.ApprovalGroupID)
	if err != nil || len(regularMembers) == 0 {
		return false
	}

	// Count active approved decisions (excluding revoked and deleted)
	var approvedCount int64
	if err := tx.Model(&models.MemberApprovalDecision{}).
		Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
		Where("member_approval_decisions.assignment_id = ? AND member_approval_decisions.deleted_at IS NULL", assignment.ID).
		Where("approval_group_members.is_final_approver = ? AND approval_group_members.is_active = ?", false, true).
		Where("member_approval_decisions.status = ?", models.DecisionApproved).
		Count(&approvedCount).Error; err != nil {
		return false
	}

	// Count any rejections (excluding revoked)
	var rejectedCount int64
	if err := tx.Model(&models.MemberApprovalDecision{}).
		Joins("JOIN approval_group_members ON approval_group_members.id = member_approval_decisions.member_id").
		Where("member_approval_decisions.assignment_id = ? AND member_approval_decisions.deleted_at IS NULL", assignment.ID).
		Where("approval_group_members.is_final_approver = ? AND approval_group_members.is_active = ?", false, true).
		Where("member_approval_decisions.status = ?", models.DecisionRejected).
		Count(&rejectedCount).Error; err != nil {
		return false
	}

	regularMemberCount := int64(len(regularMembers))

	// Ready only if:
	// - All regular members approved (count matches)
	// - No rejections exist
	// - All issues resolved (checked above)
	return approvedCount == regularMemberCount && rejectedCount == 0
}
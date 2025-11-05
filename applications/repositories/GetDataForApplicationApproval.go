package repositories

import (
	"town-planning-backend/db/models"
)

// GetDataForApplicationApproval fetches all data needed for the approval dashboard
func (r *applicationRepository) GetDataForApplicationApproval(applicationID string) (map[string]interface{}, error) {
	var application models.Application
	if err := r.db.
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Preload("ApplicationDocuments.Document").
		Preload("Payment").
		Preload("ApprovalGroup.Members.User.Department").
		Preload("ApprovalGroup.Members.User.Role").
		Preload("GroupAssignments").
		Preload("GroupAssignments.Decisions").
		Preload("GroupAssignments.Decisions.Member").
		Preload("GroupAssignments.Decisions.User").
		Preload("GroupAssignments.Decisions.User.Department").
		Preload("Issues").
		Preload("Issues.RaisedByUser").
		Preload("Issues.RaisedByUser.Department").
		Preload("Issues.AssignedToUser").
		Preload("Issues.AssignedToUser.Department").
		Preload("Issues.AssignedToGroupMember").
		Preload("Issues.AssignedToGroupMember.User").
		Preload("Issues.ResolvedByUser").
		Preload("Comments").
		Preload("Comments.User").
		Preload("Comments.User.Department").
		Preload("FinalApproval").
		Preload("FinalApproval.Approver").
		Preload("FinalApproval.Approver.Department").
		Where("id = ?", applicationID).
		First(&application).Error; err != nil {
		return nil, err
	}

	// Calculate approval progress
	approvalProgress := r.calculateApprovalProgress(&application)

	// Get workflow status
	workflowStatus := r.getWorkflowStatus(&application)

	// Transform data for frontend
	response := map[string]interface{}{
		"application":       application,
		"approval_progress": approvalProgress,
		"workflow":          workflowStatus,
		"unresolved_issues": r.countUnresolvedIssues(&application),
		"can_take_action":   r.canTakeAction(&application), // You'll need to pass current user context
	}

	return response, nil
}

// Helper method to calculate approval progress percentage
func (r *applicationRepository) calculateApprovalProgress(application *models.Application) int {
	if application.ApprovalGroup == nil || len(application.ApprovalGroup.Members) == 0 {
		return 0
	}

	// Count active members (excluding final approver for regular approval progress)
	activeMembers := 0
	approvedCount := 0

	for _, member := range application.ApprovalGroup.Members {
		if member.IsActive && !member.IsFinalApprover {
			activeMembers++
			// Check if this member has approved
			for _, assignment := range application.GroupAssignments {
				for _, decision := range assignment.Decisions {
					if decision.MemberID == member.ID && decision.Status == models.DecisionApproved {
						approvedCount++
						break
					}
				}
			}
		}
	}

	if activeMembers == 0 {
		return 0
	}

	return (approvedCount * 100) / activeMembers
}

// Helper method to get workflow status
func (r *applicationRepository) getWorkflowStatus(application *models.Application) map[string]interface{} {
	totalDepartments := 0
	approvedDepartments := 0

	if application.ApprovalGroup != nil {
		// Count unique departments in the approval group
		departmentMap := make(map[string]bool)
		for _, member := range application.ApprovalGroup.Members {
			if member.User.Department != nil {
				departmentMap[member.User.Department.ID.String()] = true
			}
		}
		totalDepartments = len(departmentMap)

		// Count approved departments
		for _, assignment := range application.GroupAssignments {
			for _, decision := range assignment.Decisions {
				if decision.Status == models.DecisionApproved && decision.User.Department != nil {
					approvedDepartments++
				}
			}
		}
	}

	return map[string]interface{}{
		"total_departments":    totalDepartments,
		"approved_departments": approvedDepartments,
		"progress_percentage":  (approvedDepartments * 100) / totalDepartments,
	}
}

// Helper method to count unresolved issues
func (r *applicationRepository) countUnresolvedIssues(application *models.Application) int {
	count := 0
	for _, issue := range application.Issues {
		if !issue.IsResolved {
			count++
		}
	}
	return count
}

// Helper method to determine if current user can take action
// You'll need to modify this to accept current user context
func (r *applicationRepository) canTakeAction(application *models.Application) bool {
	// This should check if the current user is a member of the approval group
	// and hasn't already approved/rejected
	// You'll need to pass the current user ID to this method
	return application.PaymentStatus == models.PaidPayment &&
		application.AllDocumentsProvided &&
		application.Status == models.UnderReviewApplication
}

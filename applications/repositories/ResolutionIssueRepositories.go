package repositories

import (
	"fmt"
	"time"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// repositories/application_repository.go

// MarkIssueAsResolved resolves an issue with optional resolution comment
func (r *applicationRepository) MarkIssueAsResolved(
	tx *gorm.DB,
	issueID string,
	resolvedByUserID uuid.UUID,
	resolutionComment *string,
) (*models.ApplicationIssue, error) {
	var issue models.ApplicationIssue

	// Fix: Remove the problematic Preload or use the correct relationship name
	if err := tx.
		Where("id = ?", issueID).
		First(&issue).Error; err != nil {
		return nil, fmt.Errorf("issue not found: %w", err)
	}

	if issue.IsResolved {
		return nil, fmt.Errorf("issue is already resolved")
	}

	now := time.Now()

	// Update issue resolution status
	issue.IsResolved = true
	issue.ResolvedAt = &now
	issue.ResolvedBy = &resolvedByUserID
	issue.Resolution = resolutionComment
	issue.UpdatedAt = now

	// Update the associated chat thread using direct query
	if issue.ChatThreadID != nil {
		if err := tx.Model(&models.ChatThread{}).
			Where("id = ?", *issue.ChatThreadID).
			Updates(map[string]interface{}{
				"is_resolved": true,
				"resolved_at": &now,
				"updated_at":  now,
			}).Error; err != nil {
			return nil, fmt.Errorf("failed to update chat thread: %w", err)
		}
	}

	// Save the updated issue
	if err := tx.Save(&issue).Error; err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}

	// Load the updated issue with relationships using separate queries
	var updatedIssue models.ApplicationIssue
	if err := tx.
		Preload("RaisedByUser").
		Preload("ResolvedByUser").
		Preload("AssignedToUser").
		Preload("AssignedToGroupMember").
		Preload("AssignedToGroupMember.User").
		Where("id = ?", issue.ID).
		First(&updatedIssue).Error; err != nil {
		return nil, fmt.Errorf("failed to load issue relationships: %w", err)
	}

	return &updatedIssue, nil
}

// ReopenIssue reopens a previously resolved issue
func (r *applicationRepository) ReopenIssue(
	tx *gorm.DB,
	issueID string,
	reopenedByUserID uuid.UUID,
) (*models.ApplicationIssue, error) {
	var issue models.ApplicationIssue

	if err := tx.
		Where("id = ?", issueID).
		First(&issue).Error; err != nil {
		return nil, fmt.Errorf("issue not found: %w", err)
	}

	if !issue.IsResolved {
		return nil, fmt.Errorf("issue is not resolved")
	}

	//ToDo: TEMPORARY: Bypass authorization for testing
	// // Check if user has permission to reopen
	// if !issue.CanUserResolveIssue(reopenedByUserID) {
	// 	return nil, fmt.Errorf("user not authorized to reopen this issue")
	// }

	now := time.Now()

	// Update issue resolution status
	issue.IsResolved = false
	issue.ResolvedAt = nil
	issue.ResolvedBy = nil
	issue.Resolution = nil
	issue.UpdatedAt = now

	// Update the associated chat thread using direct query
	if issue.ChatThreadID != nil {
		if err := tx.Model(&models.ChatThread{}).
			Where("id = ?", *issue.ChatThreadID).
			Updates(map[string]interface{}{
				"is_resolved": false,
				"resolved_at": nil,
				"updated_at":  now,
			}).Error; err != nil {
			return nil, fmt.Errorf("failed to update chat thread: %w", err)
		}
	}

	// Save the updated issue
	if err := tx.Save(&issue).Error; err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}

	// Load the updated issue with relationships using separate queries
	var updatedIssue models.ApplicationIssue
	if err := tx.
		Preload("RaisedByUser").
		Preload("ResolvedByUser").
		Preload("AssignedToUser").
		Preload("AssignedToGroupMember").
		Preload("AssignedToGroupMember.User").
		Where("id = ?", issue.ID).
		First(&updatedIssue).Error; err != nil {
		return nil, fmt.Errorf("failed to load issue relationships: %w", err)
	}

	return &updatedIssue, nil
}

// GetIssueByID retrieves an issue by ID with all relationships
func (r *applicationRepository) GetIssueByID(issueID string) (*models.ApplicationIssue, error) {
	var issue models.ApplicationIssue

	err := r.db.
		Preload("RaisedByUser").
		Preload("ResolvedByUser").
		Preload("AssignedToUser").
		Preload("AssignedToGroupMember").
		Preload("AssignedToGroupMember.User").
		Preload("Application").
		Preload("Assignment").
		Preload("Assignment.Group").
		Where("id = ?", issueID).
		First(&issue).Error

	if err != nil {
		return nil, err
	}

	return &issue, nil
}

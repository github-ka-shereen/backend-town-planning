package requests

import (
	"town-planning-backend/db/models"

	"github.com/google/uuid"
)

// Request types
type UnifiedParticipantRequest struct {
	Operation    string                 `json:"operation"`
	UserID       uuid.UUID              `json:"user_id,omitempty"`
	UserIDs      []uuid.UUID            `json:"user_ids,omitempty"`
	Participants []ParticipantRequest   `json:"participants,omitempty"`
	Role         models.ParticipantRole `json:"role,omitempty"`

	// Granular permissions for single add
	CanInvite *bool `json:"can_invite,omitempty"`
	CanRemove *bool `json:"can_remove,omitempty"`
	CanManage *bool `json:"can_manage,omitempty"`
}

type ParticipantRequest struct {
	UserID uuid.UUID              `json:"user_id"`
	Role   models.ParticipantRole `json:"role,omitempty"`

	// Granular permissions for bulk add
	CanInvite *bool `json:"can_invite,omitempty"`
	CanRemove *bool `json:"can_remove,omitempty"`
	CanManage *bool `json:"can_manage,omitempty"`
}

type ResolveIssueRequest struct {
	ResolutionComment *string `json:"resolution_comment" form:"resolution_comment"`
}

type ReopenIssueRequest struct {
	ReopenReason *string `json:"reopen_reason" form:"reopen_reason"`
}

type IssueResolutionResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Data    *IssueResolutionData `json:"data,omitempty"`
}

type IssueResolutionData struct {
	Issue        *models.ApplicationIssue `json:"issue"`
	ChatThreadID *uuid.UUID               `json:"chat_thread,omitempty"`
}

// RevokeDecisionRequest represents the request to revoke a decision
type RevokeDecisionRequest struct {
	Reason string `json:"reason"`
}

// RevokeDecisionResponse represents the response after revoking a decision
type RevokeDecisionResponse struct {
	Success               bool                     `json:"success"`
	Message               string                   `json:"message"`
	NewStatus             models.ApplicationStatus `json:"new_status"`
	ReadyForFinalApproval bool                     `json:"ready_for_final_approval"`
	WasFinalApprover      bool                     `json:"was_final_approver"`
	PreviousStatus        string                   `json:"previous_status"` // Added this field
}

type RevocationResult struct {
	NewStatus             models.ApplicationStatus `json:"new_status"`
	PreviousStatus        models.ApplicationStatus `json:"previous_status"` // Added this field
	WasFinalApprover      bool                     `json:"was_final_approver"`
	ReadyForFinalApproval bool                     `json:"ready_for_final_approval"`
	Message               string                   `json:"message"` // Added this field
}
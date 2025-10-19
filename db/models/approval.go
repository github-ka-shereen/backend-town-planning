package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApprovalStatus defines the status of department approvals
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "PENDING"
	ApprovalApproved ApprovalStatus = "APPROVED"
	ApprovalRejected ApprovalStatus = "REJECTED"
	ApprovalSkipped  ApprovalStatus = "SKIPPED"
)

type CommentType string

const (
    CommentTypeGeneral   CommentType = "GENERAL"
    CommentTypeApproval  CommentType = "APPROVAL"   // Approval decision reason
    CommentTypeRejection CommentType = "REJECTION"  // Rejection reason  
    CommentTypeIssue     CommentType = "ISSUE"      // Raised issue
    CommentTypeResolution CommentType = "RESOLUTION" // Issue resolution
)

// ApplicationApproval tracks each department's approval decision
type ApplicationApproval struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"application_id"`
	DepartmentID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"department_id"`
	
	// Approval details - NO COMMENTS FIELD (uses linked Comment instead)
	Status    ApprovalStatus `gorm:"type:varchar(20);default:'PENDING'" json:"status"`
	Signature string         `gorm:"not null" json:"signature"`
	
	// Approver information
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	UserName  string    `gorm:"not null" json:"user_name"`
	UserEmail string    `gorm:"not null" json:"user_email"`
	UserRole  string    `gorm:"not null" json:"user_role"`
	
	// Timestamps
	SubmittedAt *time.Time `json:"submitted_at"`
	
	// Relationships
	Application Application `gorm:"foreignKey:ApplicationID" json:"application"`
	Department  Department  `gorm:"foreignKey:DepartmentID" json:"department"`
	User        User        `gorm:"foreignKey:UserID" json:"user"`
	
	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}


// ApplicationWorkflow tracks the overall workflow state
type ApplicationWorkflow struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"application_id"`
	
	// Workflow state
	CurrentStage    string     `gorm:"not null" json:"current_stage"` // e.g., "DEPARTMENT_REVIEW", "TOWN_ENGINEER_FINAL"
	IsComplete      bool       `gorm:"default:false" json:"is_complete"`
	CompletedAt     *time.Time `json:"completed_at"`
	
	// Approval statistics
	TotalDepartments    int `gorm:"default:0" json:"total_departments"`
	ApprovedDepartments int `gorm:"default:0" json:"approved_departments"`
	RejectedDepartments int `gorm:"default:0" json:"rejected_departments"`
	PendingDepartments  int `gorm:"default:0" json:"pending_departments"`
	
	// Relationships
	Application Application          `gorm:"foreignKey:ApplicationID" json:"application"`
	Approvals   []ApplicationApproval `gorm:"foreignKey:ApplicationID" json:"approvals,omitempty"`
	
	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Comment struct {
    ID            uuid.UUID       `gorm:"type:uuid;primary_key;" json:"id"`
    ApplicationID uuid.UUID       `gorm:"type:uuid;not null;index" json:"application_id"`
    
    // Comment type for different workflow actions
    CommentType   CommentType     `gorm:"type:varchar(30);default:'GENERAL'" json:"comment_type"`
    
    // For issues and approvals
    Department    *string         `gorm:"type:varchar(30)" json:"department"`
    Subject       *string         `json:"subject"` // For issues
    Content       string          `gorm:"type:text;not null" json:"content"`
    
    // Issue tracking
    IsResolved    bool            `gorm:"default:false" json:"is_resolved"`
    ResolvedAt    *time.Time      `json:"resolved_at"`
    ResolvedBy    *string         `json:"resolved_by"`
    
    // Link to approval decision (if this comment is an approval reason)
    ApprovalID    *uuid.UUID      `gorm:"type:uuid;index" json:"approval_id"`
    
    // User info
    UserID        *uuid.UUID      `gorm:"type:uuid;index" json:"user_id"`
    CreatedBy     string          `gorm:"not null" json:"created_by"`
    
    // Other fields
    IsInternal    bool            `gorm:"default:false" json:"is_internal"`
    ParentID      *uuid.UUID      `gorm:"type:uuid;index" json:"parent_id"`
    DocumentID    *uuid.UUID      `gorm:"type:uuid;index" json:"document_id"`
    
    // Relationships
    Application Application        `gorm:"foreignKey:ApplicationID" json:"application"`
    User        User              `gorm:"foreignKey:UserID" json:"user"`
    Approval    *ApplicationApproval `gorm:"foreignKey:ApprovalID" json:"approval,omitempty"`
    
    // Audit fields
    CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApprovalGroupType defines whether the group is global or application-specific
type ApprovalGroupType string

const (
	ApprovalGroupGlobal      ApprovalGroupType = "GLOBAL"
	ApprovalGroupApplication ApprovalGroupType = "APPLICATION_SPECIFIC"
)

type CommentType string

const (
	CommentTypeGeneral    CommentType = "GENERAL"
	CommentTypeApproval   CommentType = "APPROVAL"
	CommentTypeRejection  CommentType = "REJECTION"
	CommentTypeIssue      CommentType = "ISSUE"
	CommentTypeResolution CommentType = "RESOLUTION"
)

// MemberDecisionStatus tracks individual member decisions
type MemberDecisionStatus string

const (
	DecisionPending  MemberDecisionStatus = "PENDING"
	DecisionApproved MemberDecisionStatus = "APPROVED"
	DecisionRejected MemberDecisionStatus = "REJECTED"
	DecisionRevoked  MemberDecisionStatus = "REVOKED"
	DecisionSkipped  MemberDecisionStatus = "SKIPPED"
)

// MemberRole defines the role of a group member
type MemberRole string

const (
	MemberRolePrimary MemberRole = "PRIMARY"
	MemberRoleBackup  MemberRole = "BACKUP"
	MemberRoleRetired MemberRole = "RETIRED" // Retired but history preserved
)

// AvailabilityStatus tracks member availability
type AvailabilityStatus string

const (
	AvailabilityAvailable   AvailabilityStatus = "AVAILABLE"
	AvailabilityUnavailable AvailabilityStatus = "UNAVAILABLE"
	AvailabilityLimited     AvailabilityStatus = "LIMITED" // Can handle only critical items
)

// ApprovalGroup represents a group of users who review applications
type ApprovalGroup struct {
	ID          uuid.UUID         `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string            `gorm:"type:varchar(200);not null" json:"name"`
	Description *string           `gorm:"type:text" json:"description"`
	Type        ApprovalGroupType `gorm:"type:varchar(30);not null" json:"type"`
	IsActive    bool              `gorm:"default:true;index" json:"is_active"`

	// Workflow configuration
	RequiresAllApprovals bool `gorm:"default:false" json:"requires_all_approvals"`
	MinimumApprovals     int  `gorm:"default:1" json:"minimum_approvals"`
	
	// Auto-assignment configuration
	AutoAssignBackups bool `gorm:"default:true" json:"auto_assign_backups"` // Auto-use backups when primary unavailable

	// Relationships
	Members     []ApprovalGroupMember        `gorm:"foreignKey:GroupID" json:"members,omitempty"`
	Assignments []ApplicationGroupAssignment `gorm:"foreignKey:GroupID" json:"assignments,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApprovalGroupMember represents individual members of an approval group
type ApprovalGroupMember struct {
	ID      uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	GroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"group_id"`
	UserID  uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`

	// Member role and configuration
	Role           MemberRole `gorm:"type:varchar(20);default:'PRIMARY'" json:"role"`
	IsActive       bool       `gorm:"default:true;index" json:"is_active"`
	CanRaiseIssues bool       `gorm:"default:true" json:"can_raise_issues"`
	CanApprove     bool       `gorm:"default:true" json:"can_approve"`
	CanReject      bool       `gorm:"default:true" json:"can_reject"`
	ReviewOrder    int        `gorm:"default:0" json:"review_order"`

	// Availability management
	AvailabilityStatus AvailabilityStatus `gorm:"type:varchar(20);default:'AVAILABLE'" json:"availability_status"`
	UnavailableReason  *string            `gorm:"type:text" json:"unavailable_reason"`
	UnavailableUntil   *time.Time         `json:"unavailable_until"`
	AutoReassign       bool               `gorm:"default:true" json:"auto_reassign"` // Auto-assign to backups when unavailable

	// Backup priority (lower number = higher priority)
	BackupPriority int `gorm:"default:0" json:"backup_priority"`

	// Workload limits
	MaxConcurrentReviews int `gorm:"default:10" json:"max_concurrent_reviews"`

	// Relationships
	Group ApprovalGroup `gorm:"foreignKey:GroupID" json:"group"`
	User  User          `gorm:"foreignKey:UserID" json:"user"`

	// Audit fields
	AddedBy      string         `gorm:"not null" json:"added_by"`
	AddedAt      time.Time      `gorm:"autoCreateTime" json:"added_at"`
	RemovedBy    *string        `json:"removed_by"`
	RemovedAt    *time.Time     `json:"removed_at"`
	LastActiveAt *time.Time     `json:"last_active_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApplicationGroupAssignment links applications to approval groups
type ApplicationGroupAssignment struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`
	GroupID       uuid.UUID `gorm:"type:uuid;not null;index" json:"group_id"`

	// Assignment status
	IsActive    bool       `gorm:"default:true;index" json:"is_active"`
	AssignedAt  time.Time  `gorm:"not null" json:"assigned_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// Progress tracking
	TotalMembers     int `gorm:"default:0" json:"total_members"`
	AvailableMembers int `gorm:"default:0" json:"available_members"` // Active and available members
	ApprovedCount    int `gorm:"default:0" json:"approved_count"`
	RejectedCount    int `gorm:"default:0" json:"rejected_count"`
	PendingCount     int `gorm:"default:0" json:"pending_count"`
	IssuesRaised     int `gorm:"default:0" json:"issues_raised"`
	IssuesResolved   int `gorm:"default:0" json:"issues_resolved"`

	// Backup assignment tracking
	UsedBackupMembers bool `gorm:"default:false" json:"used_backup_members"`

	// Relationships
	Application Application              `gorm:"foreignKey:ApplicationID" json:"application"`
	Group       ApprovalGroup            `gorm:"foreignKey:GroupID" json:"group"`
	Decisions   []MemberApprovalDecision `gorm:"foreignKey:AssignmentID" json:"decisions,omitempty"`

	// Audit fields
	AssignedBy string         `gorm:"not null" json:"assigned_by"`
	UpdatedBy  *string        `json:"updated_by"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// Enhanced MemberApprovalDecision with availability tracking
type MemberApprovalDecision struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	AssignmentID uuid.UUID `gorm:"type:uuid;not null;index" json:"assignment_id"`
	MemberID     uuid.UUID `gorm:"type:uuid;not null;index" json:"member_id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`

	// Decision details
	Status    MemberDecisionStatus `gorm:"type:varchar(20);default:'PENDING'" json:"status"`
	DecidedAt *time.Time           `json:"decided_at"`

	// Assignment type (primary vs backup)
	AssignedAs MemberRole `gorm:"type:varchar(20);default:'PRIMARY'" json:"assigned_as"`

	// Availability at time of assignment
	WasAvailable bool `gorm:"default:true" json:"was_available"`

	// If decision was revoked/overridden
	WasRevoked    bool       `gorm:"default:false" json:"was_revoked"`
	RevokedBy     *string    `json:"revoked_by"`
	RevokedAt     *time.Time `json:"revoked_at"`
	RevokedReason *string    `gorm:"type:text" json:"revoked_reason"`

	// Backup assignment info (if this was assigned to a backup)
	OriginalMemberID *uuid.UUID `gorm:"type:uuid;index" json:"original_member_id"` // If backup replacing someone
	BackupAssignment bool       `gorm:"default:false" json:"backup_assignment"`

	// Relationships
	Assignment     ApplicationGroupAssignment `gorm:"foreignKey:AssignmentID" json:"assignment"`
	Member         ApprovalGroupMember        `gorm:"foreignKey:MemberID" json:"member"`
	User           User                       `gorm:"foreignKey:UserID" json:"user"`
	OriginalMember *ApprovalGroupMember       `gorm:"foreignKey:OriginalMemberID" json:"original_member,omitempty"`
	Comments       []Comment                  `gorm:"foreignKey:DecisionID" json:"comments,omitempty"`
	Issues         []ApplicationIssue         `gorm:"foreignKey:RaisedByDecisionID" json:"issues,omitempty"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApplicationIssue tracks issues raised during the approval process
type ApplicationIssue struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`
	AssignmentID  uuid.UUID `gorm:"type:uuid;not null;index" json:"assignment_id"`

	// Who raised the issue
	RaisedByDecisionID uuid.UUID `gorm:"type:uuid;not null;index" json:"raised_by_decision_id"`
	RaisedByUserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"raised_by_user_id"`
	RaisedByRole       MemberRole `gorm:"type:varchar(20)" json:"raised_by_role"` // Role when issue was raised

	// Issue details
	Title       string  `gorm:"type:varchar(200);not null" json:"title"`
	Description string  `gorm:"type:text;not null" json:"description"`
	Priority    string  `gorm:"type:varchar(20);default:'MEDIUM'" json:"priority"`
	Category    *string `gorm:"type:varchar(50)" json:"category"`

	// Resolution tracking
	IsResolved bool       `gorm:"default:false;index" json:"is_resolved"`
	ResolvedAt *time.Time `json:"resolved_at"`
	ResolvedBy *uuid.UUID `gorm:"type:uuid" json:"resolved_by"`
	Resolution *string    `gorm:"type:text" json:"resolution"`

	// Relationships
	Application      Application                `gorm:"foreignKey:ApplicationID" json:"application"`
	Assignment       ApplicationGroupAssignment `gorm:"foreignKey:AssignmentID" json:"assignment"`
	RaisedByDecision MemberApprovalDecision     `gorm:"foreignKey:RaisedByDecisionID" json:"raised_by_decision"`
	RaisedByUser     User                       `gorm:"foreignKey:RaisedByUserID" json:"raised_by_user"`
	ResolvedByUser   *User                      `gorm:"foreignKey:ResolvedBy" json:"resolved_by_user,omitempty"`
	Comments         []Comment                  `gorm:"foreignKey:IssueID" json:"comments,omitempty"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// FinalApproval represents the final decision by the designated approver
type FinalApproval struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"application_id"`
	ApproverID    uuid.UUID `gorm:"type:uuid;not null;index" json:"approver_id"`

	// Final decision
	Decision   ApplicationStatus `gorm:"type:varchar(30);not null" json:"decision"`
	DecisionAt time.Time         `gorm:"not null" json:"decision_at"`
	Comments   *string           `gorm:"type:text" json:"comments"`
	Signature  string            `gorm:"not null" json:"signature"`

	// Relationships
	Application Application `gorm:"foreignKey:ApplicationID" json:"application"`
	Approver    User        `gorm:"foreignKey:ApproverID" json:"approver"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Enhanced Comment model to link with decisions and issues
type Comment struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`

	// Link to decision or issue
	DecisionID *uuid.UUID `gorm:"type:uuid;index" json:"decision_id"`
	IssueID    *uuid.UUID `gorm:"type:uuid;index" json:"issue_id"`

	// Comment details
	CommentType CommentType `gorm:"type:varchar(30);default:'GENERAL'" json:"comment_type"`
	Content     string      `gorm:"type:text;not null" json:"content"`

	// User info
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	CreatedBy string    `gorm:"not null" json:"created_by"`

	// Thread support
	ParentID *uuid.UUID `gorm:"type:uuid;index" json:"parent_id"`

	// Relationships
	Application Application             `gorm:"foreignKey:ApplicationID" json:"application"`
	Decision    *MemberApprovalDecision `gorm:"foreignKey:DecisionID" json:"decision,omitempty"`
	Issue       *ApplicationIssue       `gorm:"foreignKey:IssueID" json:"issue,omitempty"`
	User        User                    `gorm:"foreignKey:UserID" json:"user"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate hooks
func (ag *ApprovalGroup) BeforeCreate(tx *gorm.DB) error {
	if ag.ID == uuid.Nil {
		ag.ID = uuid.New()
	}
	return nil
}

func (agm *ApprovalGroupMember) BeforeCreate(tx *gorm.DB) error {
	if agm.ID == uuid.Nil {
		agm.ID = uuid.New()
	}
	// Set default backup priority based on role
	if agm.Role == MemberRolePrimary && agm.BackupPriority == 0 {
		agm.BackupPriority = 1
	}
	return nil
}

func (aga *ApplicationGroupAssignment) BeforeCreate(tx *gorm.DB) error {
	if aga.ID == uuid.Nil {
		aga.ID = uuid.New()
	}
	return nil
}

func (mad *MemberApprovalDecision) BeforeCreate(tx *gorm.DB) error {
	if mad.ID == uuid.Nil {
		mad.ID = uuid.New()
	}
	return nil
}

func (ai *ApplicationIssue) BeforeCreate(tx *gorm.DB) error {
	if ai.ID == uuid.Nil {
		ai.ID = uuid.New()
	}
	return nil
}

func (fa *FinalApproval) BeforeCreate(tx *gorm.DB) error {
	if fa.ID == uuid.Nil {
		fa.ID = uuid.New()
	}
	return nil
}

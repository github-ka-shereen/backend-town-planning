package models

import (
	"fmt"
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

// ========================================
// ISSUE ASSIGNMENT TYPES - CLEARLY DEFINED
// ========================================
type IssueAssignmentType string

const (
	// COLLABORATIVE: ANY staff member can resolve this issue
	// Use for: General questions, information gathering, collaborative problems
	// Example: "Is there paper for printing permits?" → Anyone who knows can answer
	// Fields used: AssignedToUserID = NULL, AssignedToGroupMemberID = NULL
	IssueAssignment_COLLABORATIVE IssueAssignmentType = "COLLABORATIVE"

	// GROUP_MEMBER: Only a SPECIFIC approval group member can resolve
	// Use for: Technical questions requiring specific expertise within the group
	// Example: "Verify structural engineering calculations" → Only engineering member can approve
	// Fields used: AssignedToGroupMemberID = required, AssignedToUserID = NULL
	IssueAssignment_GROUP_MEMBER IssueAssignmentType = "GROUP_MEMBER"

	// SPECIFIC_USER: Only ONE SPECIFIC staff member (anywhere in system) can resolve
	// Use for: Department-specific questions, external expertise, specialized tasks
	// Example: "Printing department - confirm paper is available" → Only printing staff member
	// Fields used: AssignedToUserID = required, AssignedToGroupMemberID = NULL
	IssueAssignment_SPECIFIC_USER IssueAssignmentType = "SPECIFIC_USER"
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
	RequiresAllApprovals bool `gorm:"default:true" json:"requires_all_approvals"`
	MinimumApprovals     int  `gorm:"default:1" json:"minimum_approvals"`

	// Auto-assignment configuration
	AutoAssignBackups bool `gorm:"default:false" json:"auto_assign_backups"`

	// Relationships
	Members     []ApprovalGroupMember        `gorm:"foreignKey:ApprovalGroupID" json:"members,omitempty"`
	Assignments []ApplicationGroupAssignment `gorm:"foreignKey:ApprovalGroupID" json:"assignments,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApprovalGroupMember represents ALL members of an approval group (including final approver)
type ApprovalGroupMember struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApprovalGroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"approval_group_id"`
	UserID          uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`

	// Member role and configuration
	Role           MemberRole `gorm:"type:varchar(20);default:'PRIMARY'" json:"role"`
	IsActive       bool       `gorm:"default:true;index" json:"is_active"`
	CanRaiseIssues bool       `gorm:"default:true" json:"can_raise_issues"`
	CanApprove     bool       `gorm:"default:true" json:"can_approve"`
	CanReject      bool       `gorm:"default:true" json:"can_reject"`
	ReviewOrder    int        `gorm:"default:0" json:"review_order"`

	// NEW: Final approver flag
	IsFinalApprover bool `gorm:"default:false;index" json:"is_final_approver"`

	// Availability management
	AvailabilityStatus AvailabilityStatus `gorm:"type:varchar(20);default:'AVAILABLE'" json:"availability_status"`
	UnavailableReason  *string            `gorm:"type:text" json:"unavailable_reason"`
	UnavailableUntil   *time.Time         `json:"unavailable_until"`
	AutoReassign       bool               `gorm:"default:true" json:"auto_reassign"` // Auto-assign to backups when unavailable

	// Backup priority (lower number = higher priority)
	BackupPriority int `gorm:"default:0" json:"backup_priority"`

	// Relationships
	ApprovalGroup ApprovalGroup `gorm:"foreignKey:ApprovalGroupID" json:"approval_group"`
	User          User          `gorm:"foreignKey:UserID" json:"user"`

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
	ID              uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID   uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`
	ApprovalGroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"approval_group_id"`

	// Assignment status
	IsActive    bool       `gorm:"default:true;index" json:"is_active"`
	AssignedAt  time.Time  `gorm:"not null" json:"assigned_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// Progress tracking
	TotalMembers     int `gorm:"default:0" json:"total_members"`
	AvailableMembers int `gorm:"default:0" json:"available_members"`
	ApprovedCount    int `gorm:"default:0" json:"approved_count"`
	RejectedCount    int `gorm:"default:0" json:"rejected_count"`
	PendingCount     int `gorm:"default:0" json:"pending_count"`
	IssuesRaised     int `gorm:"default:0" json:"issues_raised"`
	IssuesResolved   int `gorm:"default:0" json:"issues_resolved"`

	// Final approver tracking
	ReadyForFinalApproval   bool       `gorm:"default:false" json:"ready_for_final_approval"`
	FinalApproverAssignedAt *time.Time `json:"final_approver_assigned_at"`
	FinalDecisionAt         *time.Time `json:"final_decision_at"`
	FinalDecisionID         *uuid.UUID `gorm:"type:uuid;index" json:"final_decision_id"` // ← ADD THIS

	// Backup assignment tracking
	UsedBackupMembers bool `gorm:"default:false" json:"used_backup_members"`

	// Relationships
	Application   Application              `gorm:"foreignKey:ApplicationID" json:"application"`
	Group         ApprovalGroup            `gorm:"foreignKey:ApprovalGroupID" json:"group"`
	Decisions     []MemberApprovalDecision `gorm:"foreignKey:AssignmentID" json:"decisions,omitempty"`
	FinalDecision *FinalApproval           `gorm:"foreignKey:FinalDecisionID" json:"final_decision,omitempty"` // ← FIX THIS

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

	// NEW: Track if this is a final approver decision
	IsFinalApproverDecision bool `gorm:"default:false" json:"is_final_approver_decision"`

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

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApplicationIssue tracks issues raised during the approval process
type ApplicationIssue struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`
	AssignmentID  uuid.UUID `gorm:"type:uuid;not null;index" json:"assignment_id"` // Links to ApplicationGroupAssignment

	// ========================================
	// WHO RAISED THE ISSUE
	// ========================================
	// Link to the decision record where this issue was raised
	// This gives us access to all decision context (member, role, status, etc.)

	// Direct link to user for quick queries (denormalized for performance)
	// User details (name, email, etc.) come from User table relationship
	RaisedByUserID uuid.UUID `gorm:"type:uuid;not null;index" json:"raised_by_user_id"`

	// ========================================
	// WHO CAN RESOLVE THE ISSUE
	// ========================================
	// Controls the resolution permissions
	AssignmentType IssueAssignmentType `gorm:"type:varchar(30);default:'COLLABORATIVE';not null" json:"assignment_type"`

	// For SPECIFIC_USER mode: Points to any user in system
	// NULL for COLLABORATIVE and GROUP_MEMBER modes
	AssignedToUserID *uuid.UUID `gorm:"type:uuid;index" json:"assigned_to_user_id"`

	// For GROUP_MEMBER mode: Points to specific approval group member
	// NULL for COLLABORATIVE and SPECIFIC_USER modes
	AssignedToGroupMemberID *uuid.UUID `gorm:"type:uuid;index" json:"assigned_to_group_member_id"`

	// Chat thread reference
	ChatThreadID *uuid.UUID `gorm:"type:uuid;index" json:"chat_thread_id"`

	// ========================================
	// ISSUE DETAILS
	// ========================================
	Title       string  `gorm:"type:varchar(200);not null" json:"title"`
	Description string  `gorm:"type:text;not null" json:"description"`
	Priority    string  `gorm:"type:varchar(20);default:'MEDIUM'" json:"priority"` // LOW, MEDIUM, HIGH, CRITICAL
	Category    *string `gorm:"type:varchar(50)" json:"category"`                  // Optional: LOGISTICS, TECHNICAL, ADMINISTRATIVE, etc.

	// ========================================
	// RESOLUTION TRACKING
	// ========================================
	IsResolved bool       `gorm:"default:false;index" json:"is_resolved"`
	ResolvedAt *time.Time `json:"resolved_at"`
	ResolvedBy *uuid.UUID `gorm:"type:uuid;index" json:"resolved_by"` // Which user resolved it
	Resolution *string    `gorm:"type:text" json:"resolution"`        // Resolution details

	// ========================================
	// RELATIONSHIPS (NORMALIZED)
	// ========================================
	// Main entities
	Application Application                `gorm:"foreignKey:ApplicationID" json:"application"`
	Assignment  ApplicationGroupAssignment `gorm:"foreignKey:AssignmentID" json:"assignment"`

	// Who raised the issue
	RaisedByGroupMemberID uuid.UUID           `gorm:"type:uuid;not null;index" json:"raised_by_group_member_id"`
	RaisedByUser          User                `gorm:"foreignKey:RaisedByUserID" json:"raised_by_user"`
	RaisedByGroupMember   ApprovalGroupMember `gorm:"foreignKey:RaisedByGroupMemberID" json:"raised_by_group_member"`

	// Who can/did resolve
	AssignedToUser        *User                `gorm:"foreignKey:AssignedToUserID" json:"assigned_to_user,omitempty"`
	AssignedToGroupMember *ApprovalGroupMember `gorm:"foreignKey:AssignedToGroupMemberID" json:"assigned_to_group_member,omitempty"`
	ResolvedByUser        *User                `gorm:"foreignKey:ResolvedBy" json:"resolved_by_user,omitempty"`

	// Comments on this issue
	Comments []Comment `gorm:"foreignKey:IssueID" json:"comments,omitempty"`

	// ========================================
	// AUDIT FIELDS
	// ========================================
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ========================================
// HELPER METHODS FOR VALIDATION
// ========================================

// FinalApproval represents the final decision by the designated approver
type FinalApproval struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"application_id"`
	// AssignmentID  *uuid.UUID `gorm:"type:uuid;index" json:"assignment_id"` // ← REMOVE THIS LINE
	ApproverID    uuid.UUID `gorm:"type:uuid;not null;index" json:"approver_id"`

	// Final decision
	Decision   ApplicationStatus `gorm:"type:varchar(30);not null" json:"decision"`
	DecisionAt time.Time         `gorm:"not null" json:"decision_at"`
	Comment    *string           `gorm:"type:text" json:"comment"`

	// Override information (if final approver overrode group decision)
	OverrodeGroupDecision bool    `gorm:"default:false" json:"overrode_group_decision"`
	OverrideReason        *string `gorm:"type:text" json:"override_reason"`

	// Relationships
	Application Application `gorm:"foreignKey:ApplicationID" json:"application"`
	// Assignment  *ApplicationGroupAssignment `gorm:"foreignKey:AssignmentID" json:"assignment,omitempty"` // ← REMOVE THIS LINE
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

	CommentDocuments []CommentDocument `gorm:"foreignKey:CommentID" json:"comment_documents,omitempty"`

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

// DecisionRevocation tracks when a decision is revoked
type DecisionRevocation struct {
	ID             uuid.UUID            `gorm:"type:uuid;primary_key;" json:"id"`
	DecisionID     uuid.UUID            `gorm:"type:uuid;not null;index" json:"decision_id"`
	RevokedBy      uuid.UUID            `gorm:"type:uuid;not null" json:"revoked_by"`
	Reason         string               `gorm:"type:text;not null" json:"reason"`
	RevokedAt      time.Time            `gorm:"not null" json:"revoked_at"`
	PreviousStatus MemberDecisionStatus `gorm:"type:varchar(20)" json:"previous_status"`

	// Relationships
	Decision MemberApprovalDecision `gorm:"foreignKey:DecisionID" json:"decision"`
	Revoker  User                   `gorm:"foreignKey:RevokedBy" json:"revoker"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate hook
func (dr *DecisionRevocation) BeforeCreate(tx *gorm.DB) error {
	if dr.ID == uuid.Nil {
		dr.ID = uuid.New()
	}
	return nil
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

func (c *Comment) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return
}

// Helper method to check if all regular members have approved
func (aga *ApplicationGroupAssignment) AllRegularMembersApproved() bool {
	// Count only non-final-approver members
	regularMembersCount := 0
	for _, member := range aga.Group.Members {
		if !member.IsFinalApprover {
			regularMembersCount++
		}
	}

	if aga.Group.RequiresAllApprovals {
		return aga.ApprovedCount >= regularMembersCount
	}
	return aga.ApprovedCount >= aga.Group.MinimumApprovals
}

// Helper method to check if application is ready for final approval
func (aga *ApplicationGroupAssignment) IsReadyForFinalApproval() bool {
	return aga.AllRegularMembersApproved() && aga.IssuesRaised == aga.IssuesResolved
}

// Helper method to get the final approver member
func (ag *ApprovalGroup) GetFinalApprover() *ApprovalGroupMember {
	for _, member := range ag.Members {
		if member.IsFinalApprover && member.IsActive {
			return &member
		}
	}
	return nil
}

// Helper method to get regular members (non-final approvers)
func (ag *ApprovalGroup) GetRegularMembers() []ApprovalGroupMember {
	var regularMembers []ApprovalGroupMember
	for _, member := range ag.Members {
		if !member.IsFinalApprover && member.IsActive {
			regularMembers = append(regularMembers, member)
		}
	}
	return regularMembers
}

// CanUserResolveIssue checks if a user has permission to resolve this issue
func (issue *ApplicationIssue) CanUserResolveIssue(userID uuid.UUID) bool {
	if issue.IsResolved {
		return false // Already resolved
	}

	// Allow the issue creator to always resolve their own issues
	if issue.RaisedByUserID == userID {
		return true
	}

	switch issue.AssignmentType {
	case IssueAssignment_COLLABORATIVE:
		return true

	case IssueAssignment_GROUP_MEMBER:
		if issue.AssignedToGroupMemberID == nil {
			return false
		}
		return issue.AssignedToGroupMember != nil && issue.AssignedToGroupMember.UserID == userID

	case IssueAssignment_SPECIFIC_USER:
		if issue.AssignedToUserID == nil {
			return false
		}
		return *issue.AssignedToUserID == userID

	default:
		return false
	}
}

// GetRequiredResolver returns information about who needs to resolve this issue
func (issue *ApplicationIssue) GetRequiredResolver() string {
	switch issue.AssignmentType {
	case IssueAssignment_COLLABORATIVE:
		return "Any staff member can resolve"
	case IssueAssignment_GROUP_MEMBER:
		if issue.AssignedToGroupMember != nil && issue.AssignedToGroupMember.UserID != uuid.Nil {
			return fmt.Sprintf("Only %s %s (Group Member) can resolve",
				issue.AssignedToGroupMember.User.FirstName,
				issue.AssignedToGroupMember.User.LastName)
		}
		return "Specific group member required"
	case IssueAssignment_SPECIFIC_USER:
		if issue.AssignedToUser != nil {
			return fmt.Sprintf("Only %s %s can resolve",
				issue.AssignedToUser.FirstName,
				issue.AssignedToUser.LastName)
		}
		return "Specific user required"
	default:
		return "Unknown assignment type"
	}
}

// ValidateAssignment ensures the assignment is properly configured
func (issue *ApplicationIssue) ValidateAssignment() error {
	switch issue.AssignmentType {
	case IssueAssignment_COLLABORATIVE:
		// Should not have specific assignments
		if issue.AssignedToUserID != nil || issue.AssignedToGroupMemberID != nil {
			return fmt.Errorf("COLLABORATIVE issues cannot have specific assignments")
		}

	case IssueAssignment_GROUP_MEMBER:
		// Must have group member assignment
		if issue.AssignedToGroupMemberID == nil {
			return fmt.Errorf("GROUP_MEMBER issues require AssignedToGroupMemberID")
		}
		if issue.AssignedToUserID != nil {
			return fmt.Errorf("GROUP_MEMBER issues cannot have AssignedToUserID")
		}

	case IssueAssignment_SPECIFIC_USER:
		// Must have user assignment
		if issue.AssignedToUserID == nil {
			return fmt.Errorf("SPECIFIC_USER issues require AssignedToUserID")
		}
		if issue.AssignedToGroupMemberID != nil {
			return fmt.Errorf("SPECIFIC_USER issues cannot have AssignedToGroupMemberID")
		}

	default:
		return fmt.Errorf("invalid assignment type: %s", issue.AssignmentType)
	}

	return nil
}

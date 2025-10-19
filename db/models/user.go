package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AuthMethod string

const (
	AuthMethodMagicLink     AuthMethod = "magic_link"
	AuthMethodPassword      AuthMethod = "password"
	AuthMethodAuthenticator AuthMethod = "authenticator"
)

// Permission represents individual system permissions
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"name" validate:"required,min=3,max=100"`
	Description string    `gorm:"type:varchar(500)" json:"description" validate:"max=500"`
	Resource    string    `gorm:"type:varchar(50);not null;index:idx_permission_resource" json:"resource" validate:"required,min=2,max=50"`
	Action      string    `gorm:"type:varchar(20);not null;index:idx_permission_action" json:"action" validate:"required,oneof=create read update delete"`
	Category    string    `gorm:"type:varchar(50);index:idx_permission_category" json:"category" validate:"max=50"`
	IsActive    bool      `gorm:"default:true;index:idx_permission_active" json:"is_active"`

	// Relationships
	RolePermissions []RolePermission `gorm:"foreignKey:PermissionID;constraint:OnDelete:CASCADE" json:"role_permissions,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"type:varchar(255);not null" json:"created_by" validate:"required"`
	CreatedAt time.Time      `gorm:"autoCreateTime;index:idx_created_at" json:"created_at"`
	UpdatedBy *string        `gorm:"type:varchar(255)" json:"updated_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Permission
func (Permission) TableName() string {
	return "permissions"
}

// Role represents a dynamic user role with permissions
type Role struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"name" validate:"required,min=2,max=100"`
	Description string    `gorm:"type:varchar(500)" json:"description" validate:"max=500"`
	IsSystem    bool      `gorm:"default:false;index:idx_role_system" json:"is_system"`
	IsActive    bool      `gorm:"default:true;index:idx_role_active" json:"is_active"`

	// Relationships
	Permissions []RolePermission `gorm:"foreignKey:RoleID;constraint:OnDelete:CASCADE" json:"permissions,omitempty"`
	Users       []User           `gorm:"foreignKey:RoleID;constraint:OnDelete:RESTRICT" json:"users,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"type:varchar(255);not null" json:"created_by" validate:"required"`
	CreatedAt time.Time      `gorm:"autoCreateTime;index:idx_created_at" json:"created_at"`
	UpdatedBy *string        `gorm:"type:varchar(255)" json:"updated_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Role
func (Role) TableName() string {
	return "roles"
}

// RolePermission join table for role permissions with unique constraint
type RolePermission struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	RoleID       uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_role_permission_unique" json:"role_id"`
	PermissionID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_role_permission_unique" json:"permission_id"`

	// Relationships with proper constraints
	Role       Role       `gorm:"foreignKey:RoleID;constraint:OnDelete:CASCADE" json:"role,omitempty"`
	Permission Permission `gorm:"foreignKey:PermissionID;constraint:OnDelete:CASCADE" json:"permission,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for RolePermission
func (RolePermission) TableName() string {
	return "role_permissions"
}

// Department represents a dynamic department
type Department struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"name" validate:"required,min=2,max=100"`
	Description *string   `gorm:"type:varchar(500)" json:"description" validate:"max=500"`
	IsActive    bool      `gorm:"default:true;index:idx_department_active" json:"is_active"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"`

	// Contact information with proper validation
	Email          *string `gorm:"type:varchar(255)" json:"email" validate:"omitempty,email"`
	PhoneNumber    *string `gorm:"type:varchar(20)" json:"phone_number" validate:"omitempty,e164"`
	OfficeLocation *string `gorm:"type:varchar(200)" json:"office_location" validate:"omitempty,max=200"`

	// Relationships
	Users []User `gorm:"foreignKey:DepartmentID;constraint:OnDelete:SET NULL" json:"users,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"type:varchar(255);not null" json:"created_by" validate:"required"`
	CreatedAt time.Time      `gorm:"autoCreateTime;index:idx_created_at" json:"created_at"`
	UpdatedBy *string        `gorm:"type:varchar(255)" json:"updated_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Department
func (Department) TableName() string {
	return "departments"
}

// User represents system users with dynamic roles
type User struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	FirstName      string     `gorm:"type:varchar(100);not null" json:"first_name" validate:"required,min=2,max=100"`
	LastName       string     `gorm:"type:varchar(100);not null" json:"last_name" validate:"required,min=2,max=100"`
	Email          string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"email" validate:"required,email"`
	Phone          string     `gorm:"type:varchar(20);uniqueIndex" json:"phone" validate:"omitempty,e164"`
	WhatsAppNumber *string    `gorm:"type:varchar(20)" json:"whatsapp_number" validate:"omitempty,e164"`
	Password       string     `gorm:"type:text;column:password" json:"-"`
	AuthMethod     AuthMethod `gorm:"type:varchar(20);default:'magic_link'" json:"auth_method" validate:"oneof=magic_link password authenticator"`
	TOTPSecret     string     `gorm:"type:text;column:totp_secret" json:"-"`

	// Dynamic role and department with proper constraints
	RoleID       uuid.UUID  `gorm:"type:uuid;not null;index:idx_user_role;constraint:OnDelete:RESTRICT" json:"role_id" validate:"required"`
	DepartmentID *uuid.UUID `gorm:"type:uuid;index:idx_user_department;constraint:OnDelete:SET NULL" json:"department_id"`

	// Status fields with indexes for common queries
	Active        bool `gorm:"default:true;index:idx_user_active" json:"active"`
	IsSuspended   bool `gorm:"default:false;index:idx_user_suspended" json:"is_suspended"`
	EmailVerified bool `gorm:"default:false;index:idx_user_email_verified" json:"email_verified"`

	// Security fields
	FailedLoginAttempts int        `gorm:"default:0" json:"-"`
	LockedUntil         *time.Time `json:"-"`
	LastLoginAt         *time.Time `json:"last_login_at,omitempty"`
	PasswordChangedAt   *time.Time `json:"-"`

	// Profile information
	ProfilePictureURL *string `gorm:"type:varchar(500)" json:"profile_picture_url" validate:"omitempty,url"`
	SignatureFilePath *string `gorm:"type:varchar(500)" json:"signature_file_path"`

	// Audit fields (using custom names for User model)
	CreatedBy     string         `gorm:"type:varchar(255);not null" json:"created_by" validate:"required"`
	CreatedAt     time.Time      `gorm:"autoCreateTime;index:idx_user_created" json:"created_at"`
	UpdatedBy     *string        `gorm:"type:varchar(255)" json:"updated_by,omitempty"`
	LastUpdatedAt time.Time      `gorm:"autoUpdateTime" json:"last_updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Role       Role           `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Department *Department    `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	AuditLogs  []UserAuditLog `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"audit_logs,omitempty"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}

// IsLocked checks if the user account is currently locked
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

// ShouldLockAccount determines if account should be locked based on failed attempts
func (u *User) ShouldLockAccount() bool {
	return u.FailedLoginAttempts >= 5 // configurable threshold
}

// UserAuditLog tracks changes made to users with enhanced fields
type UserAuditLog struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;not null;index:idx_audit_user;constraint:OnDelete:CASCADE" json:"user_id"`

	// Changed fields tracking with better structure
	ChangedFields datatypes.JSON `gorm:"type:json" json:"changed_fields"`
	OldValues     datatypes.JSON `gorm:"type:json" json:"old_values"`
	NewValues     datatypes.JSON `gorm:"type:json" json:"new_values"`

	// Enhanced audit information
	ChangedBy    string  `gorm:"type:varchar(255);not null" json:"changed_by" validate:"required"`
	IPAddress    *string `gorm:"type:varchar(45)" json:"ip_address" validate:"omitempty,ip"`
	UserAgent    *string `gorm:"type:varchar(500)" json:"user_agent"`
	ActionType   string  `gorm:"type:varchar(20);index:idx_audit_action" json:"action_type" validate:"required,oneof=create update delete login logout"`
	ResourceType string  `gorm:"type:varchar(50);index:idx_audit_resource" json:"resource_type" validate:"required"`
	SessionID    *string `gorm:"type:varchar(255)" json:"session_id"`

	// Relationships
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`

	// Timestamps
	CreatedAt time.Time `gorm:"autoCreateTime;index:idx_audit_created" json:"created_at"`
}

// TableName specifies the table name for UserAuditLog
func (UserAuditLog) TableName() string {
	return "user_audit_logs"
}

// Enhanced Hooks with validation
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}

	// Basic validation
	if u.Email == "" {
		return errors.New("email is required")
	}
	if u.FirstName == "" {
		return errors.New("first name is required")
	}
	if u.LastName == "" {
		return errors.New("last name is required")
	}
	if u.RoleID == uuid.Nil {
		return errors.New("role is required")
	}

	return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
	// Set UpdatedBy if it's in the context
	if updatedBy := tx.Statement.Context.Value("updated_by"); updatedBy != nil {
		if updatedByStr, ok := updatedBy.(string); ok {
			u.UpdatedBy = &updatedByStr
		}
	}
	return nil
}

func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}

	if r.Name == "" {
		return errors.New("role name is required")
	}

	return nil
}

func (p *Permission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	if p.Name == "" {
		return errors.New("permission name is required")
	}
	if p.Resource == "" {
		return errors.New("permission resource is required")
	}
	if p.Action == "" {
		return errors.New("permission action is required")
	}

	return nil
}

func (d *Department) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}

	if d.Name == "" {
		return errors.New("department name is required")
	}

	return nil
}

func (rp *RolePermission) BeforeCreate(tx *gorm.DB) error {
	if rp.ID == uuid.Nil {
		rp.ID = uuid.New()
	}

	if rp.RoleID == uuid.Nil {
		return errors.New("role ID is required")
	}
	if rp.PermissionID == uuid.Nil {
		return errors.New("permission ID is required")
	}

	return nil
}

func (ual *UserAuditLog) BeforeCreate(tx *gorm.DB) error {
	if ual.ID == uuid.Nil {
		ual.ID = uuid.New()
	}

	if ual.UserID == uuid.Nil {
		return errors.New("user ID is required")
	}
	if ual.ActionType == "" {
		return errors.New("action type is required")
	}
	if ual.ChangedBy == "" {
		return errors.New("changed by is required")
	}

	return nil
}

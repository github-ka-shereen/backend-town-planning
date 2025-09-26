package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DocumentType string

const (
	PDFDocument         DocumentType = "PDF"
	ImageDocument       DocumentType = "IMAGE"
	WordDocument        DocumentType = "WORD_DOCUMENT"
	SpreadsheetDocument DocumentType = "SPREADSHEET"
	CADDocument         DocumentType = "CAD_DRAWING"
	SurveyPlanDocument  DocumentType = "SURVEY_PLAN"
)

// ActionType defines the type of action performed on a document
type ActionType string

const (
	ActionCreate  ActionType = "CREATE"
	ActionUpdate  ActionType = "UPDATE"
	ActionReplace ActionType = "REPLACE"
	ActionDelete  ActionType = "DELETE"
	ActionRestore ActionType = "RESTORE"
)

// DocumentCategory represents document categories (will be seeded)
type DocumentCategory struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null;unique" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"` // System categories cannot be modified
	IsActive    bool      `gorm:"default:true" json:"is_active"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedBy string         `gorm:"not null" json:"created_by"`
}

// Document represents uploaded files associated with applicants or applications
type Document struct {
	ID           uuid.UUID    `gorm:"type:uuid;primary_key;" json:"id"`
	DocumentCode *string      `gorm:"index" json:"document_code"`
	FileName     string       `gorm:"not null" json:"file_name"`
	DocumentType DocumentType `gorm:"type:varchar(30);not null" json:"document_type"`
	CategoryID   uuid.UUID    `gorm:"type:uuid;not null;index" json:"category_id"` // Required category reference
	FileSize     int64        `gorm:"not null" json:"file_size"`
	FilePath     string       `gorm:"not null" json:"file_path"`
	FileHash     string       `gorm:"index" json:"file_hash"`
	MimeType     string       `json:"mime_type"`
	IsPublic     bool         `gorm:"default:false" json:"is_public"`
	Description  *string      `gorm:"type:text" json:"description"`
	IsMandatory  bool         `gorm:"default:true" json:"is_mandatory"`
	IsActive     bool         `gorm:"default:true" json:"is_active"`

	// Associations
	ApplicantID   *uuid.UUID `gorm:"type:uuid;index" json:"applicant_id"`
	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id"`
	StandID       *uuid.UUID `gorm:"type:uuid;index" json:"stand_id"`
	ProjectID     *uuid.UUID `gorm:"type:uuid;index" json:"project_id"`

	// Enhanced Version Control - FIXED: Self-referencing within Document table
	Version          int        `gorm:"default:1" json:"version"`
	PreviousID       *uuid.UUID `gorm:"type:uuid;index" json:"previous_id"` // References another Document
	OriginalID       *uuid.UUID `gorm:"type:uuid;index" json:"original_id"` // References another Document
	IsCurrentVersion bool       `gorm:"default:true;index" json:"is_current_version"`

	// Update tracking
	UpdateReason *string    `gorm:"type:text" json:"update_reason"`
	UpdatedBy    *string    `json:"updated_by"` // Who made the last update
	LastAction   ActionType `gorm:"type:varchar(20);default:'CREATE'" json:"last_action"`

	// Relationships - FIXED: Proper self-referencing constraints
	Applicant   *Applicant   `gorm:"foreignKey:ApplicantID" json:"applicant,omitempty"`
	Application *Application `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Stand       *Stand       `gorm:"foreignKey:StandID" json:"stand,omitempty"`
	Project     *Project     `gorm:"foreignKey:ProjectID" json:"project,omitempty"`

	// Self-referencing relationships within Document table
	Previous *Document `gorm:"foreignKey:PreviousID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"previous,omitempty"`
	Original *Document `gorm:"foreignKey:OriginalID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"original,omitempty"`

	// Reverse relationships
	Newer    []Document        `gorm:"foreignKey:PreviousID" json:"newer,omitempty"`    // Documents that have this one as previous
	Versions []Document        `gorm:"foreignKey:OriginalID" json:"versions,omitempty"` // All versions of this document
	Category *DocumentCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`

	// Audit trail relationship
	AuditLogs []DocumentAuditLog `gorm:"foreignKey:DocumentID" json:"audit_logs,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// DocumentAuditLog tracks all changes made to documents
type DocumentAuditLog struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	DocumentID uuid.UUID  `gorm:"type:uuid;not null;index" json:"document_id"`
	Action     ActionType `gorm:"type:varchar(20);not null" json:"action"`
	UserID     string     `gorm:"not null" json:"user_id"`
	UserName   *string    `json:"user_name"`
	UserRole   *string    `json:"user_role"`
	Reason     *string    `gorm:"type:text" json:"reason"`
	Details    *string    `gorm:"type:text" json:"details"`
	IPAddress  *string    `json:"ip_address"`
	UserAgent  *string    `json:"user_agent"`

	// Document state before/after change
	OldFileName    *string    `json:"old_file_name"`
	OldCategoryID  *uuid.UUID `json:"old_category_id"`
	OldDescription *string    `json:"old_description"`
	OldIsPublic    *bool      `json:"old_is_public"`
	OldIsMandatory *bool      `json:"old_is_mandatory"`
	OldIsActive    *bool      `json:"old_is_active"`

	NewFileName    *string    `json:"new_file_name"`
	NewCategoryID  *uuid.UUID `json:"new_category_id"`
	NewDescription *string    `json:"new_description"`
	NewIsPublic    *bool      `json:"new_is_public"`
	NewIsMandatory *bool      `json:"new_is_mandatory"`
	NewIsActive    *bool      `json:"new_is_active"`

	// Relationship
	Document *Document `gorm:"foreignKey:DocumentID" json:"document,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type DocumentVersion struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	BaseDocumentID   uuid.UUID `gorm:"type:uuid;not null;index" json:"base_document_id"`
	DocumentID       uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	Version          int       `gorm:"not null" json:"version"`
	FileName         string    `gorm:"not null" json:"file_name"`
	FileSize         int64     `gorm:"not null" json:"file_size"`
	FileHash         string    `gorm:"not null" json:"file_hash"`
	IsCurrentVersion bool      `gorm:"not null;index" json:"is_current_version"`
	CreatedBy        string    `gorm:"not null" json:"created_by"`
	UpdateReason     *string   `gorm:"type:text" json:"update_reason"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`

	Document *Document `gorm:"foreignKey:DocumentID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"document,omitempty"`
	Base     *Document `gorm:"foreignKey:BaseDocumentID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"base,omitempty"`
}

// BeforeCreate hooks for UUID generation
func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}

	// Set original ID for the first version to point to itself
	if d.OriginalID == nil {
		d.OriginalID = &d.ID
	}

	return nil
}

func (dc *DocumentCategory) BeforeCreate(tx *gorm.DB) error {
	if dc.ID == uuid.Nil {
		dc.ID = uuid.New()
	}
	return nil
}

func (dal *DocumentAuditLog) BeforeCreate(tx *gorm.DB) error {
	if dal.ID == uuid.Nil {
		dal.ID = uuid.New()
	}
	return nil
}

func (dv *DocumentVersion) BeforeCreate(tx *gorm.DB) error {
	if dv.ID == uuid.Nil {
		dv.ID = uuid.New()
	}
	return nil
}

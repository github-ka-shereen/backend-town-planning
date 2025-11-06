package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Approval actions
type Action string

const (
	ActionCreate  Action = "CREATE"
	ActionApprove Action = "APPROVE"
	ActionReject  Action = "REJECT"
	ActionDelete  Action = "DELETE"
	ActionUpdate  Action = "UPDATE"
	ActionRestore Action = "RESTORE"
	ActionReplace Action = "REPLACE"
	ActionPending Action = "PENDING"
	ActionRevise  Action = "REVERSE"
)

// DocumentType with housing-specific types
type DocumentType string

const (
	WordDocumentType       DocumentType = "WORD_DOCUMENT"
	TextDocumentType       DocumentType = "TEXT_DOCUMENT"
	SpreadsheetType        DocumentType = "SPREADSHEET"
	PresentationType       DocumentType = "PRESENTATION"
	ImageType              DocumentType = "IMAGE"
	PDFType                DocumentType = "PDF"
	CADDrawingType         DocumentType = "CAD_DRAWING"
	SurveyPlanType         DocumentType = "SURVEY_PLAN"
	EngineeringCertificate DocumentType = "ENGINEERING_CERTIFICATE"
	BuildingPlanType       DocumentType = "BUILDING_PLAN"
	SitePlanType           DocumentType = "SITE_PLAN"
)

// DocumentCategory represents document categories
type DocumentCategory struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null;unique" json:"name"`
	Code        string    `gorm:"type:varchar(50);not null;unique" json:"code"`
	Description string    `gorm:"type:text" json:"description"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedBy string         `gorm:"not null" json:"created_by"`
}

// Document model - CLEANED UP with only core fields and versioning
type Document struct {
	ID           uuid.UUID       `gorm:"type:uuid;primary_key;" json:"id"`
	DocumentCode *string         `gorm:"index" json:"document_code"`
	FileName     string          `json:"file_name"`
	DocumentType DocumentType    `json:"document_type"`
	FileSize     decimal.Decimal `json:"file_size"`
	CategoryID   *uuid.UUID      `gorm:"type:uuid" json:"category_id"`

	// File storage details
	FilePath string `json:"file_path"`
	FileHash string `gorm:"index" json:"file_hash"`
	MimeType string `json:"mime_type"`

	// Document metadata
	Description *string `gorm:"type:text" json:"description"`
	IsPublic    bool    `gorm:"default:false" json:"is_public"`
	IsMandatory bool    `gorm:"default:true" json:"is_mandatory"`
	IsActive    bool    `gorm:"default:true" json:"is_active"`

	// Version Control
	Version          int        `gorm:"default:1" json:"version"`
	PreviousID       *uuid.UUID `gorm:"type:uuid;index" json:"previous_id"`
	OriginalID       *uuid.UUID `gorm:"type:uuid;index" json:"original_id"`
	IsCurrentVersion bool       `gorm:"default:true;index" json:"is_current_version"`

	// Update tracking
	UpdateReason *string `gorm:"type:text" json:"update_reason"`
	UpdatedBy    *string `json:"updated_by"`
	LastAction   Action  `gorm:"type:varchar(20);default:'CREATE'" json:"last_action"`

	// Relationships - KEEP ONLY category and versioning relationships
	Category *DocumentCategory `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"category,omitempty"`
	Previous *Document         `gorm:"foreignKey:PreviousID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"previous,omitempty"`
	Original *Document         `gorm:"foreignKey:OriginalID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"original,omitempty"`

	// Reverse relationships
	Newer     []Document         `gorm:"foreignKey:PreviousID" json:"newer,omitempty"`
	Versions  []Document         `gorm:"foreignKey:OriginalID" json:"versions,omitempty"`
	AuditLogs []DocumentAuditLog `gorm:"foreignKey:DocumentID" json:"audit_logs,omitempty"`

	// NEW: Reverse relationships to join tables (for querying convenience)
	ApplicantDocuments   []ApplicantDocument   `gorm:"foreignKey:DocumentID" json:"applicant_documents,omitempty"`
	ApplicationDocuments []ApplicationDocument `gorm:"foreignKey:DocumentID" json:"application_documents,omitempty"`
	StandDocuments       []StandDocument       `gorm:"foreignKey:DocumentID" json:"stand_documents,omitempty"`
	ProjectDocuments     []ProjectDocument     `gorm:"foreignKey:DocumentID" json:"project_documents,omitempty"`
	CommentDocuments     []CommentDocument     `gorm:"foreignKey:DocumentID" json:"comment_documents,omitempty"`
	PaymentDocuments     []PaymentDocument     `gorm:"foreignKey:DocumentID" json:"payment_documents,omitempty"`
	EmailDocuments       []EmailDocument       `gorm:"foreignKey:DocumentID" json:"email_documents,omitempty"`
	BankDocuments        []BankDocument        `gorm:"foreignKey:DocumentID" json:"bank_documents,omitempty"`
	UserDocuments        []UserDocument        `gorm:"foreignKey:DocumentID" json:"user_documents,omitempty"`
	ChatAttachments      []ChatAttachment      `gorm:"foreignKey:DocumentID" json:"chat_attachments,omitempty"`
	
	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// DocumentAuditLog tracks all changes made to documents
type DocumentAuditLog struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	Action     Action    `gorm:"type:varchar(20);not null" json:"action"`
	UserID     string    `gorm:"not null" json:"user_id"`
	UserName   *string   `json:"user_name"`
	UserRole   *string   `json:"user_role"`
	Reason     *string   `gorm:"type:text" json:"reason"`
	Details    *string   `gorm:"type:text" json:"details"`
	IPAddress  *string   `json:"ip_address"`
	UserAgent  *string   `json:"user_agent"`

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

// ====================
// JOIN TABLE MODELS
// ====================

// ApplicantDocument represents the relationship between applicants and documents
type ApplicantDocument struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicantID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"applicant_id"`
	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id"`
	DocumentID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Applicant   Applicant    `gorm:"foreignKey:ApplicantID;constraint:OnDelete:CASCADE" json:"applicant"`
	Application *Application `gorm:"foreignKey:ApplicationID;constraint:OnDelete:CASCADE" json:"application,omitempty"`
	Document    Document     `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
}

// ApplicationDocument represents the relationship between applications and documents
type ApplicationDocument struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`
	DocumentID    uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Application Application `gorm:"foreignKey:ApplicationID;constraint:OnDelete:CASCADE" json:"application"`
	Document    Document    `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
}

// StandDocument represents the relationship between stands and documents
type StandDocument struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	StandID    uuid.UUID `gorm:"type:uuid;not null;index" json:"stand_id"`
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Stand    Stand    `gorm:"foreignKey:StandID;constraint:OnDelete:CASCADE" json:"stand"`
	Document Document `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
}

// ProjectDocument represents the relationship between projects and documents
type ProjectDocument struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ProjectID  uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Project  Project  `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE" json:"project"`
	Document Document `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
}

// PaymentPlanDocument represents the relationship between payment plans and documents
type CommentDocument struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	CommentID  uuid.UUID `gorm:"type:uuid;not null;index" json:"comment_id"`
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Comment  Comment  `gorm:"foreignKey:CommentID;constraint:OnDelete:CASCADE" json:"comment"`
	Document Document `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
}

type BankDocument struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	BankID     uuid.UUID `gorm:"type:uuid;not null;index" json:"bank_id"`
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Bank     Bank     `gorm:"foreignKey:BankID;constraint:OnDelete:CASCADE" json:"bank"`
	Document Document `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
}

type UserDocument struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	User     User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user"`
	Document Document `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
}

// ====================
// Business Logic Methods
// ====================

// Get document display name
func (d *Document) GetDisplayName() string {
	if d.Description != nil && *d.Description != "" {
		return *d.Description
	}
	return d.FileName
}

// Check if document is the latest version
func (d *Document) IsLatestVersion() bool {
	return d.IsCurrentVersion
}

// Get version information
func (d *Document) GetVersionInfo() string {
	if d.OriginalID != nil && *d.OriginalID != d.ID {
		return fmt.Sprintf("Version %d of document", d.Version)
	}
	return "Original document"
}

// Check if document can be updated
func (d *Document) CanBeUpdated() bool {
	return d.IsActive && d.IsCurrentVersion
}

// Check if document can be deleted
func (d *Document) CanBeDeleted() bool {
	// Can only delete if no other versions depend on it
	return len(d.Newer) == 0 && d.IsCurrentVersion
}

// Get file extension
func (d *Document) GetFileExtension() string {
	if len(d.FileName) > 0 {
		if idx := strings.LastIndex(d.FileName, "."); idx != -1 {
			return d.FileName[idx+1:]
		}
	}
	return ""
}

// Check if document is an image
func (d *Document) IsImage() bool {
	return d.DocumentType == ImageType
}

// Check if document is a PDF
func (d *Document) IsPDF() bool {
	return d.DocumentType == PDFType
}

// Get file size in human readable format
func (d *Document) GetHumanReadableSize() string {
	size := d.FileSize.InexactFloat64()
	const unit = 1000
	if size < unit {
		return fmt.Sprintf("%.0f B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", size/float64(div), "kMGTPE"[exp])
}

// BeforeCreate hooks
func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
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

func (appDoc *ApplicationDocument) BeforeCreate(tx *gorm.DB) error {
	if appDoc.ID == uuid.Nil {
		appDoc.ID = uuid.New()
	}
	return nil
}

func (sd *StandDocument) BeforeCreate(tx *gorm.DB) error {
	if sd.ID == uuid.Nil {
		sd.ID = uuid.New()
	}
	return nil
}

func (pd *ProjectDocument) BeforeCreate(tx *gorm.DB) error {
	if pd.ID == uuid.Nil {
		pd.ID = uuid.New()
	}
	return nil
}

func (cd *CommentDocument) BeforeCreate(tx *gorm.DB) error {
	if cd.ID == uuid.Nil {
		cd.ID = uuid.New()
	}
	return nil
}

// Add BeforeCreate hooks
func (bd *BankDocument) BeforeCreate(tx *gorm.DB) error {
	if bd.ID == uuid.Nil {
		bd.ID = uuid.New()
	}
	return nil
}

func (ud *UserDocument) BeforeCreate(tx *gorm.DB) error {
	if ud.ID == uuid.Nil {
		ud.ID = uuid.New()
	}
	return nil
}

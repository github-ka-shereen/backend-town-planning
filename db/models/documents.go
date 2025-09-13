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

// Predefined document categories
type PredefinedCategory string

const (
	// Legal and Identity Documents
	TitleDeedCategory                PredefinedCategory = "TITLE_DEED"
	IDCopyCategory                   PredefinedCategory = "ID_COPY"
	OrganisationRegistrationCategory PredefinedCategory = "ORGANISATION_REGISTRATION"
	PowerOfAttorneyCategory          PredefinedCategory = "POWER_OF_ATTORNEY"

	// Planning Documents
	BuildingPlansCategory         PredefinedCategory = "BUILDING_PLANS"
	SurveyPlanCategory            PredefinedCategory = "SURVEY_PLAN"
	SiteLayoutCategory            PredefinedCategory = "SITE_LAYOUT"
	ArchitecturalDrawingsCategory PredefinedCategory = "ARCHITECTURAL_DRAWINGS"
	StructuralDrawingsCategory    PredefinedCategory = "STRUCTURAL_DRAWINGS"

	// Financial Documents
	PaymentReceiptCategory  PredefinedCategory = "PAYMENT_RECEIPT"
	RatesClearanceCategory  PredefinedCategory = "RATES_CLEARANCE"
	AgreementOfSaleCategory PredefinedCategory = "AGREEMENT_OF_SALE"

	// Technical Certificates
	EngineeringCertificateCategory PredefinedCategory = "ENGINEERING_CERTIFICATE"
	LimpimCertificateCategory      PredefinedCategory = "LIMPIM_CERTIFICATE"
	EnvironmentalClearanceCategory PredefinedCategory = "ENVIRONMENTAL_CLEARANCE"

	// Application Forms
	TPDFormCategory         PredefinedCategory = "TPD_FORM"
	ApplicationFormCategory PredefinedCategory = "APPLICATION_FORM"

	// Communication
	CorrespondenceCategory PredefinedCategory = "CORRESPONDENCE"
	NotificationCategory   PredefinedCategory = "NOTIFICATION"

	// Other
	OtherDocumentCategory PredefinedCategory = "OTHER"
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

// Document represents uploaded files associated with applicants or applications
type Document struct {
	ID                 uuid.UUID          `gorm:"type:uuid;primary_key;" json:"id"`
	DocumentCode       *string            `gorm:"index" json:"document_code"`
	FileName           string             `gorm:"not null" json:"file_name"`
	DocumentType       DocumentType       `gorm:"type:varchar(30);not null" json:"document_type"`
	CategoryID         *uuid.UUID         `gorm:"type:uuid;index" json:"category_id"`                // Reference to custom category
	PredefinedCategory PredefinedCategory `gorm:"type:varchar(50);index" json:"predefined_category"` // For predefined categories
	FileSize           int64              `gorm:"not null" json:"file_size"`
	FilePath           string             `gorm:"not null" json:"file_path"`
	FileHash           string             `gorm:"index" json:"file_hash"`
	MimeType           string             `json:"mime_type"`
	IsPublic           bool               `gorm:"default:false" json:"is_public"`
	Description        *string            `gorm:"type:text" json:"description"`
	IsMandatory        bool               `gorm:"default:true" json:"is_mandatory"`
	IsActive           bool               `gorm:"default:true" json:"is_active"`

	// Associations
	ApplicantID   *uuid.UUID `gorm:"type:uuid;index" json:"applicant_id"`
	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id"`

	// Enhanced Version Control
	Version          int        `gorm:"default:1" json:"version"`
	PreviousID       *uuid.UUID `gorm:"type:uuid;index" json:"previous_id"`
	OriginalID       *uuid.UUID `gorm:"type:uuid;index" json:"original_id"` // Points to the first version
	IsCurrentVersion bool       `gorm:"default:true;index" json:"is_current_version"`

	// Update tracking
	UpdateReason *string    `gorm:"type:text" json:"update_reason"`
	UpdatedBy    *string    `json:"updated_by"` // Who made the last update
	LastAction   ActionType `gorm:"type:varchar(20);default:'CREATE'" json:"last_action"`

	// Relationships - CORRECTED
	Applicant   *Applicant      `gorm:"foreignKey:ApplicantID" json:"applicant,omitempty"`
	Application *Application    `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Previous    *Document       `gorm:"foreignKey:PreviousID" json:"previous,omitempty"`
	Newer       []Document      `gorm:"foreignKey:PreviousID" json:"newer,omitempty"` // Documents that have this one as previous
	Category    *CustomCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Original    *Document       `gorm:"foreignKey:OriginalID" json:"original,omitempty"`

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
	OldFileName           *string             `json:"old_file_name"`
	OldPredefinedCategory *PredefinedCategory `json:"old_predefined_category"`
	OldDescription        *string             `json:"old_description"`
	OldIsPublic           *bool               `json:"old_is_public"`
	OldIsMandatory        *bool               `json:"old_is_mandatory"`
	OldIsActive           *bool               `json:"old_is_active"`

	NewFileName           *string             `json:"new_file_name"`
	NewPredefinedCategory *PredefinedCategory `json:"new_predefined_category"`
	NewDescription        *string             `json:"new_description"`
	NewIsPublic           *bool               `json:"new_is_public"`
	NewIsMandatory        *bool               `json:"new_is_mandatory"`
	NewIsActive           *bool               `json:"new_is_active"`

	// Relationship - CORRECTED
	Document *Document `gorm:"foreignKey:DocumentID" json:"document,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// DocumentVersion provides a consolidated view of document versions
type DocumentVersion struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	OriginalID       uuid.UUID `gorm:"type:uuid;not null;index" json:"original_id"`
	DocumentID       uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	Version          int       `gorm:"not null" json:"version"`
	FileName         string    `gorm:"not null" json:"file_name"`
	FileSize         int64     `gorm:"not null" json:"file_size"`
	FileHash         string    `gorm:"not null" json:"file_hash"`
	IsCurrentVersion bool      `gorm:"not null;index" json:"is_current_version"`
	CreatedBy        string    `gorm:"not null" json:"created_by"`
	UpdateReason     *string   `gorm:"type:text" json:"update_reason"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships - CORRECTED
	Document *Document `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
	Original *Document `gorm:"foreignKey:OriginalID" json:"original,omitempty"`
}

// CustomCategory represents user-defined document categories
type CustomCategory struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`

	// Relationships - REMOVED incorrect PredefinedCategory reference
	// If you need to link to predefined categories, you should have a separate field
	// PredefinedCategoryID *uuid.UUID `gorm:"type:uuid;index" json:"predefined_category_id"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedBy string         `gorm:"not null" json:"created_by"`
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}

	// Set original ID for the first version
	if d.OriginalID == nil {
		d.OriginalID = &d.ID
	}

	return nil
}

func (d *Document) BeforeUpdate(tx *gorm.DB) error {
	// This will be called automatically by GORM before updates
	// You can add custom logic here if needed
	return nil
}

func (c *CustomCategory) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
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

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

type DocumentCategory string

const (
	// Legal and Identity Documents
	TitleDeedCategory                DocumentCategory = "TITLE_DEED"
	IDCopyCategory                   DocumentCategory = "ID_COPY"
	OrganisationRegistrationCategory DocumentCategory = "ORGANISATION_REGISTRATION"
	PowerOfAttorneyCategory          DocumentCategory = "POWER_OF_ATTORNEY"

	// Planning Documents
	BuildingPlansCategory         DocumentCategory = "BUILDING_PLANS"
	SurveyPlanCategory            DocumentCategory = "SURVEY_PLAN"
	SiteLayoutCategory            DocumentCategory = "SITE_LAYOUT"
	ArchitecturalDrawingsCategory DocumentCategory = "ARCHITECTURAL_DRAWINGS"
	StructuralDrawingsCategory    DocumentCategory = "STRUCTURAL_DRAWINGS"

	// Financial Documents
	PaymentReceiptCategory  DocumentCategory = "PAYMENT_RECEIPT"
	RatesClearanceCategory  DocumentCategory = "RATES_CLEARANCE"
	AgreementOfSaleCategory DocumentCategory = "AGREEMENT_OF_SALE"

	// Technical Certificates
	EngineeringCertificateCategory DocumentCategory = "ENGINEERING_CERTIFICATE"
	LimpimCertificateCategory      DocumentCategory = "LIMPIM_CERTIFICATE"
	EnvironmentalClearanceCategory DocumentCategory = "ENVIRONMENTAL_CLEARANCE"

	// Application Forms
	TPDFormCategory         DocumentCategory = "TPD_FORM"
	ApplicationFormCategory DocumentCategory = "APPLICATION_FORM"

	// Communication
	CorrespondenceCategory DocumentCategory = "CORRESPONDENCE"
	NotificationCategory   DocumentCategory = "NOTIFICATION"

	// Other
	OtherDocumentCategory DocumentCategory = "OTHER"
)

// Document represents uploaded files associated with applicants or applications
type Document struct {
	ID               uuid.UUID        `gorm:"type:uuid;primary_key;" json:"id"`
	FileName         string           `gorm:"not null" json:"file_name"`
	OriginalFileName string           `gorm:"not null" json:"original_file_name"`
	DocumentType     DocumentType     `gorm:"type:varchar(30);not null" json:"document_type"`
	DocumentCategory DocumentCategory `gorm:"type:varchar(50);not null;index" json:"document_category"`
	FileSize         int64            `gorm:"not null" json:"file_size"` // in bytes
	FilePath         string           `gorm:"not null" json:"file_path"`
	FileHash         string           `gorm:"index" json:"file_hash"` // For duplicate detection
	MimeType         string           `json:"mime_type"`
	IsPublic         bool             `gorm:"default:false" json:"is_public"`
	Description      *string          `gorm:"type:text" json:"description"`

	// Associations
	ApplicantID   *uuid.UUID `gorm:"type:uuid;index" json:"applicant_id"`
	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id"`

	// Version control
	Version    int        `gorm:"default:1" json:"version"`
	PreviousID *uuid.UUID `gorm:"type:uuid;index" json:"previous_id"` // Links to previous version

	// Relationships
	Applicant   *Applicant   `gorm:"foreignKey:ApplicantID;constraint:OnDelete:CASCADE" json:"applicant,omitempty"`
	Application *Application `gorm:"foreignKey:ApplicationID;constraint:OnDelete:CASCADE" json:"application,omitempty"`
	Previous    *Document    `gorm:"foreignKey:PreviousID" json:"previous,omitempty"`
	Newer       []Document   `gorm:"foreignKey:PreviousID" json:"newer,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

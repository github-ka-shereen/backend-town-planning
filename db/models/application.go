package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ApplicationStatus defines the current state of an application.
type ApplicationStatus string

const (
	SubmittedApplication   ApplicationStatus = "SUBMITTED"
	UnderReviewApplication ApplicationStatus = "UNDER_REVIEW"
	ApprovedApplication    ApplicationStatus = "APPROVED"
	RejectedApplication    ApplicationStatus = "REJECTED"
	CollectedApplication   ApplicationStatus = "COLLECTED"
	ExpiredApplication     ApplicationStatus = "EXPIRED"
)

type PermitStatus string

const (
	PermitActive    PermitStatus = "ACTIVE"
	PermitExpired   PermitStatus = "EXPIRED"
	PermitRevoked   PermitStatus = "REVOKED"
	PermitSuspended PermitStatus = "SUSPENDED"
)

// DevelopmentCategory model for dynamic development categories
type DevelopmentCategory struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"unique;not null" json:"name"` // e.g., "COMMERCIAL & INDUSTRIAL", "MEDIUM & LOW DENSITY", "CHURCH STRUCTURES" appropriate for the stand type
	Description *string   `json:"description"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"` // System types cannot be modified
	IsActive    bool      `gorm:"default:true" json:"is_active"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedBy string         `gorm:"not null" json:"created_by"`
}

// Permit model for issued permits
type Permit struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	PermitNumber  string    `gorm:"unique;not null;index" json:"permit_number"` // e.g., "VFCC/PERMIT/2024/001"
	ApplicationID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_id"`

	// Permit details
	IssueDate  time.Time    `gorm:"not null" json:"issue_date"`
	ValidUntil *time.Time   `json:"valid_until"` // Typically issueDate + 24 months
	Status     PermitStatus `gorm:"type:varchar(20);default:'ACTIVE'" json:"status"`

	// Development category this permit was issued for
	DevelopmentCategoryID uuid.UUID `gorm:"type:uuid;not null;index" json:"development_category_id"`

	// Relationships
	Application         Application         `gorm:"foreignKey:ApplicationID" json:"application"`
	DevelopmentCategory DevelopmentCategory `gorm:"foreignKey:DevelopmentCategoryID" json:"development_category"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedBy string         `gorm:"not null" json:"created_by"`
}

// Tariff defines the FEES for a development category during a specific period
type Tariff struct {
	ID                     uuid.UUID       `gorm:"type:uuid;primary_key;" json:"id"`
	DevelopmentCategoryID  uuid.UUID       `gorm:"type:uuid;not null;index" json:"development_category_id"`
	Currency               string          `gorm:"type:varchar(10);not null" json:"currency"` // e.g., USD, ZWL
	PricePerSquareMeter    decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"price_per_square_meter"`
	PermitFee              decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"permit_fee"`
	InspectionFee          decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"inspection_fee"`
	DevelopmentLevyPercent decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"development_levy_percent"` // e.g., 15.00 = 15%
	ValidFrom              time.Time       `gorm:"not null;index" json:"valid_from"`
	ValidTo                *time.Time      `gorm:"index" json:"valid_to"` // NULL means currently active
	IsActive               bool            `gorm:"default:true" json:"is_active"`

	// Relationships
	DevelopmentCategory DevelopmentCategory `gorm:"foreignKey:DevelopmentCategoryID" json:"development_category"`
	Payments            []Payment           `gorm:"foreignKey:TariffID" json:"payments"` // Link to related payments (optional)

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// VATRate model with validity period
type VATRate struct {
	ID        uuid.UUID       `gorm:"type:uuid;primary_key;" json:"id"`
	Rate      decimal.Decimal `gorm:"type:decimal(5,2);not null" json:"rate"` // Store as percentage (e.g., 0.15 for 15%)
	ValidFrom time.Time       `gorm:"not null;index" json:"valid_from"`
	ValidTo   *time.Time      `gorm:"index" json:"valid_to"` // NULL means currently active
	IsActive  bool            `gorm:"default:true" json:"is_active"`
	Used      bool            `gorm:"default:false" json:"used"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	UpdatedBy *string        `json:"updated_by"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Application struct {
	ID                   uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	PlanNumber           string    `gorm:"unique;not null;index" json:"plan_number"`
	PermitNumber         string    `gorm:"unique;not null;index" json:"permit_number"`
	ArchitectFullName    *string   `json:"architect_full_name"`
	ArchitectEmail       *string   `json:"architect_email"`
	ArchitectPhoneNumber *string   `json:"architect_phone_number"`

	// Planning details
	PlanArea *decimal.Decimal `gorm:"type:decimal(15,2)" json:"plan_area"`

	// Financial summary
	DevelopmentLevy *decimal.Decimal `gorm:"type:decimal(15,2)" json:"development_levy"`
	VATAmount       *decimal.Decimal `gorm:"type:decimal(15,2)" json:"vat_amount"`
	TotalCost       *decimal.Decimal `gorm:"type:decimal(15,2)" json:"total_cost"`
	EstimatedCost   *decimal.Decimal `gorm:"type:decimal(15,2)" json:"estimated_cost"`

	// Payment tracking (overall, not individual receipts)
	PaymentStatus PaymentStatus `gorm:"type:varchar(20);default:'PENDING'" json:"payment_status"`

	// Status and dates
	Status         ApplicationStatus `gorm:"type:varchar(20);default:'SUBMITTED';index" json:"status"`
	SubmissionDate time.Time         `gorm:"not null" json:"submission_date"`
	ApprovalDate   *time.Time        `json:"approval_date"`
	RejectionDate  *time.Time        `json:"rejection_date"`
	CollectionDate *time.Time        `json:"collection_date"`

	// Collection tracking
	IsCollected bool    `gorm:"default:false" json:"is_collected"`
	CollectedBy *string `json:"collected_by"`

	// Document verification flags
	ScannedReceiptProvided                   bool `gorm:"default:false" json:"scanned_receipt_provided"`
	ScannedInitialPlanProvided               bool `gorm:"default:false" json:"scanned_initial_plan_provided"`
	ScannedTPD1FormProvided                  bool `gorm:"default:false" json:"scanned_tpd1_form_provided"`
	QuotationProvided                        bool `gorm:"default:false" json:"quotation_provided"`
	StructuralEngineeringCertificateProvided bool `gorm:"default:false" json:"structural_engineering_certificate_provided"`
	RingBeamCertificateProvided              bool `gorm:"default:false" json:"ring_beam_certificate_provided"`

	// Property details
	PropertyTypeID *uuid.UUID `gorm:"type:uuid;index" json:"property_type_id"`
	StandID        *string    `gorm:"index" json:"stand_id"`
	Stand          *Stand     `gorm:"foreignKey:StandID" json:"stand,omitempty"`
	ApplicantID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"applicant_id"`

	// Reference to the actual rates used for this application
	TariffID  *uuid.UUID `gorm:"type:uuid;index" json:"tariff_id"`
	VATRateID *uuid.UUID `gorm:"type:uuid;index" json:"vat_rate_id"`

	// Relationships
	Applicant Applicant  `gorm:"foreignKey:ApplicantID" json:"applicant"`
	Tariff    *Tariff    `gorm:"foreignKey:TariffID" json:"tariff,omitempty"`
	VATRate   *VATRate   `gorm:"foreignKey:VATRateID" json:"vat_rate,omitempty"`
	Documents []Document `gorm:"foreignKey:ApplicationID" json:"documents,omitempty"`
	Comments  []Comment  `gorm:"foreignKey:ApplicationID" json:"comments,omitempty"`
	Payment   Payment    `gorm:"foreignKey:ApplicationID" json:"payment,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Comment model remains the same
type Comment struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"application_id"`
	Department    *string    `gorm:"type:varchar(30)" json:"department"`
	UserID        *uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Subject       *string    `json:"subject"`
	Content       string     `gorm:"type:text;not null" json:"content"`
	IsInternal    bool       `gorm:"default:false" json:"is_internal"`
	IsResolved    bool       `gorm:"default:false" json:"is_resolved"`
	IsActive      bool       `gorm:"default:true" json:"is_active"`
	ParentID      *uuid.UUID `gorm:"type:uuid;index" json:"parent_id"`
	DocumentID    *uuid.UUID `gorm:"type:uuid;index" json:"document_id"`

	// Relationships
	Application Application `gorm:"foreignKey:ApplicationID" json:"application"`
	Parent      *Comment    `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Replies     []Comment   `gorm:"foreignKey:ParentID" json:"replies,omitempty"`
	User        User        `gorm:"foreignKey:UserID" json:"user"`
	Document    Document    `gorm:"foreignKey:DocumentID" json:"document"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Application
func (a *Application) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return
}

// Comment
func (c *Comment) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return
}

// DevelopmentCategory
func (pt *DevelopmentCategory) BeforeCreate(tx *gorm.DB) (err error) {
	if pt.ID == uuid.Nil {
		pt.ID = uuid.New()
	}
	return
}

// Permit
func (p *Permit) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return
}

// Tariff
func (t *Tariff) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return
}

// VATRate
func (v *VATRate) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return
}

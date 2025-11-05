package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type PermitStatus string

const (
	PermitActive    PermitStatus = "ACTIVE"
	PermitExpired   PermitStatus = "EXPIRED"
	PermitRevoked   PermitStatus = "REVOKED"
	PermitSuspended PermitStatus = "SUSPENDED"
)

// ApplicationStatus defines the current state of an application (ADDED THIS)
type ApplicationStatus string

const (
	SubmittedApplication          ApplicationStatus = "SUBMITTED"
	UnderReviewApplication        ApplicationStatus = "UNDER_REVIEW"
	PendingApprovalApplication    ApplicationStatus = "PENDING_APPROVAL"
	ApprovedApplication           ApplicationStatus = "APPROVED"
	RejectedApplication           ApplicationStatus = "REJECTED"
	CollectedApplication          ApplicationStatus = "COLLECTED"
	ExpiredApplication            ApplicationStatus = "EXPIRED"
	DepartmentReviewApplication   ApplicationStatus = "DEPARTMENT_REVIEW"
	FinalReviewApplication        ApplicationStatus = "FINAL_REVIEW"
	ReadyForCollectionApplication ApplicationStatus = "READY_FOR_COLLECTION"
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
	ID           uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	PlanNumber   string    `gorm:"unique;not null;index" json:"plan_number"`
	PermitNumber string    `gorm:"unique;index" json:"permit_number"` // Generated after final approval

	// Architect details
	ArchitectFullName    *string `json:"architect_full_name"`
	ArchitectEmail       *string `json:"architect_email"`
	ArchitectPhoneNumber *string `json:"architect_phone_number"`

	// Planning details
	PlanArea *decimal.Decimal `gorm:"type:decimal(15,2)" json:"plan_area"`

	// Financial summary
	DevelopmentLevy *decimal.Decimal `gorm:"type:decimal(15,2)" json:"development_levy"`
	VATAmount       *decimal.Decimal `gorm:"type:decimal(15,2)" json:"vat_amount"`
	TotalCost       *decimal.Decimal `gorm:"type:decimal(15,2)" json:"total_cost"`
	EstimatedCost   *decimal.Decimal `gorm:"type:decimal(15,2)" json:"estimated_cost"`

	// Payment and document prerequisites for approval group review
	PaymentStatus        PaymentStatus `gorm:"type:varchar(20);default:'PENDING'" json:"payment_status"`
	AllDocumentsProvided bool          `gorm:"default:false;index" json:"all_documents_provided"`
	ReadyForReview       bool          `gorm:"default:false;index" json:"ready_for_review"` // Payment complete + docs provided

	// Application workflow status
	Status         ApplicationStatus `gorm:"type:varchar(40);default:'SUBMITTED';index" json:"status"`
	SubmissionDate time.Time         `gorm:"not null" json:"submission_date"`

	// Workflow milestone dates
	PaymentCompletedAt   *time.Time `json:"payment_completed_at"`
	DocumentsCompletedAt *time.Time `json:"documents_completed_at"`
	ReviewStartedAt      *time.Time `json:"review_started_at"`
	ReviewCompletedAt    *time.Time `json:"review_completed_at"`
	FinalApprovalDate    *time.Time `json:"final_approval_date"`
	RejectionDate        *time.Time `json:"rejection_date"`
	CollectionDate       *time.Time `json:"collection_date"`

	// Collection tracking
	IsCollected bool    `gorm:"default:false" json:"is_collected"`
	CollectedBy *string `json:"collected_by"`

	// Document verification flags
	ProcessedReceiptProvided                 bool `gorm:"default:false" json:"processed_receipt_provided"`
	InitialPlanProvided                      bool `gorm:"default:false" json:"initial_plan_provided"`
	ProcessedTPD1FormProvided                bool `gorm:"default:false" json:"processed_tpd1_form_provided"`
	ProcessedQuotationProvided               bool `gorm:"default:false" json:"processed_quotation_provided"`
	StructuralEngineeringCertificateProvided bool `gorm:"default:false" json:"structural_engineering_certificate_provided"`
	RingBeamCertificateProvided              bool `gorm:"default:false" json:"ring_beam_certificate_provided"`

	// Property details
	PropertyTypeID *uuid.UUID `gorm:"type:uuid;index" json:"property_type_id"`
	StandID        *uuid.UUID `gorm:"type:uuid;index" json:"stand_id"`
	ApplicantID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"applicant_id"`

	// Tariff references
	TariffID  *uuid.UUID `gorm:"type:uuid;index" json:"tariff_id"`
	VATRateID *uuid.UUID `gorm:"type:uuid;index" json:"vat_rate_id"`

	// Approval group assignment (can be null if using global group)
	AssignedGroupID *uuid.UUID     `gorm:"type:uuid;index" json:"assigned_group_id"`
	ApprovalGroup   *ApprovalGroup `gorm:"foreignKey:AssignedGroupID" json:"approval_group,omitempty"`

	// Final approver assignment
	FinalApproverID *uuid.UUID `gorm:"type:uuid;index" json:"final_approver_id"`

	// Relationships
	Applicant            Applicant             `gorm:"foreignKey:ApplicantID" json:"applicant"`
	Tariff               *Tariff               `gorm:"foreignKey:TariffID" json:"tariff,omitempty"`
	VATRate              *VATRate              `gorm:"foreignKey:VATRateID" json:"vat_rate,omitempty"`
	Stand                *Stand                `gorm:"foreignKey:StandID" json:"stand,omitempty"`
	ApplicationDocuments []ApplicationDocument `gorm:"foreignKey:ApplicationID" json:"application_documents,omitempty"`
	Payment              Payment               `gorm:"foreignKey:ApplicationID" json:"payment,omitempty"`

	// New approval group relationships
	GroupAssignments []ApplicationGroupAssignment `gorm:"foreignKey:ApplicationID" json:"group_assignments,omitempty"`
	Issues           []ApplicationIssue           `gorm:"foreignKey:ApplicationID" json:"issues,omitempty"`
	Comments         []Comment                    `gorm:"foreignKey:ApplicationID" json:"comments,omitempty"`
	FinalApproval    *FinalApproval               `gorm:"foreignKey:ApplicationID" json:"final_approval,omitempty"`
	FinalApprover    *User                        `gorm:"foreignKey:FinalApproverID" json:"final_approver,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
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

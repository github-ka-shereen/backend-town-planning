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

type PropertyType string

const (
	IndustrialProperty       PropertyType = "INDUSTRIAL"
	CommercialProperty       PropertyType = "COMMERCIAL"
	ChurchProperty           PropertyType = "CHURCH"
	HighDensityResidential   PropertyType = "HIGH_DENSITY_RESIDENTIAL"
	MediumDensityResidential PropertyType = "MEDIUM_DENSITY_RESIDENTIAL"
	LowDensityResidential    PropertyType = "LOW_DENSITY_RESIDENTIAL"
	HolidayHomeProperty      PropertyType = "HOLIDAY_HOME"
	GovernmentProperty       PropertyType = "GOVERNMENT"
	EducationalProperty      PropertyType = "EDUCATIONAL"
	HealthcareProperty       PropertyType = "HEALTHCARE"
	RecreationalProperty     PropertyType = "RECREATIONAL"
)

type PaymentStatus string

const (
	PendingPayment   PaymentStatus = "PENDING"
	PaidPayment      PaymentStatus = "PAID"
	PartialPayment   PaymentStatus = "PARTIAL"
	RefundedPayment  PaymentStatus = "REFUNDED"
	CancelledPayment PaymentStatus = "CANCELLED"
)

type CommentType string

const (
	DepartmentComment    CommentType = "DEPARTMENT_COMMENT"
	ApprovalComment      CommentType = "APPROVAL"
	RejectionComment     CommentType = "REJECTION"
	InformationalComment CommentType = "INFORMATIONAL"
	ClientResponseComment CommentType = "CLIENT_RESPONSE"
)

// Application represents a single submission to the town planning system.
type Application struct {
	ID                    uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationNumber     string    `gorm:"unique;not null;index" json:"application_number"`
	ClientID              uuid.UUID `gorm:"type:uuid;not null;index" json:"client_id"`
	ApplicationCategoryID uuid.UUID `gorm:"type:uuid;not null;index" json:"application_category_id"`

	// Property details
	PropertyType    *PropertyType `gorm:"type:varchar(30)" json:"property_type"`
	StandNumber     *string       `gorm:"index" json:"stand_number"`
	ERFNumber       *string       `gorm:"index" json:"erf_number"`
	PlanNumber      *string       `gorm:"index" json:"plan_number"`
	PropertyAddress *string       `json:"property_address"`

	// Planning details
	PlanArea           *decimal.Decimal `gorm:"type:decimal(15,2)" json:"plan_area"`
	ProposedUse        *string          `json:"proposed_use"`
	NumberOfUnits      *int             `json:"number_of_units"`
	NumberOfStories    *int             `json:"number_of_stories"`
	ProjectDescription *string          `gorm:"type:text" json:"project_description"`

	// Financial calculations
	PricePerSquareMeter *decimal.Decimal `gorm:"type:decimal(15,2)" json:"price_per_square_meter"`
	PermitFee           *decimal.Decimal `gorm:"type:decimal(15,2)" json:"permit_fee"`
	InspectionFee       *decimal.Decimal `gorm:"type:decimal(15,2)" json:"inspection_fee"`
	DevelopmentLevy     *decimal.Decimal `gorm:"type:decimal(15,2)" json:"development_levy"`
	VATAmount           *decimal.Decimal `gorm:"type:decimal(15,2)" json:"vat_amount"`
	TotalCost           *decimal.Decimal `gorm:"type:decimal(15,2)" json:"total_cost"`

	// Payment tracking
	PaymentStatus PaymentStatus    `gorm:"type:varchar(20);default:'PENDING'" json:"payment_status"`
	ReceiptNumber *string          `gorm:"index" json:"receipt_number"`
	PaymentDate   *time.Time       `json:"payment_date"`
	AmountPaid    *decimal.Decimal `gorm:"type:decimal(15,2)" json:"amount_paid"`

	// Status and dates
	Status          ApplicationStatus `gorm:"type:varchar(20);default:'SUBMITTED';index" json:"status"`
	SubmissionDate  time.Time         `gorm:"not null" json:"submission_date"`
	ReviewStartDate *time.Time        `json:"review_start_date"`
	ApprovalDate    *time.Time        `json:"approval_date"`
	RejectionDate   *time.Time        `json:"rejection_date"`
	CollectionDate  *time.Time        `json:"collection_date"`
	ExpiryDate      *time.Time        `json:"expiry_date"`

	// Collection tracking
	IsCollected     bool    `gorm:"default:false" json:"is_collected"`
	CollectedBy     *string `json:"collected_by"`
	CollectionNotes *string `gorm:"type:text" json:"collection_notes"`

	// Internal notes
	InternalNotes   *string `gorm:"type:text" json:"internal_notes"`
	RejectionReason *string `gorm:"type:text" json:"rejection_reason"`

	// Relationships
	Client              Client              `gorm:"foreignKey:ClientID;constraint:OnDelete:RESTRICT" json:"client"`
	ApplicationCategory ApplicationCategory `gorm:"foreignKey:ApplicationCategoryID;constraint:OnDelete:RESTRICT" json:"application_category"`
	Documents           []Document          `gorm:"foreignKey:ApplicationID" json:"documents,omitempty"`
	Comments            []Comment           `gorm:"foreignKey:ApplicationID" json:"comments,omitempty"`
	Reviews             []ApplicationReview `gorm:"foreignKey:ApplicationID" json:"reviews,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApplicationCategory represents configurable application types
type ApplicationCategory struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"unique;not null" json:"name"`
	Code        string    `gorm:"unique;not null;size:10" json:"code"` // For numbering scheme
	Description string    `gorm:"type:text" json:"description"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`

	// Fee configuration
	HasBaseFee          bool             `gorm:"default:false" json:"has_base_fee"`
	BaseFee             *decimal.Decimal `gorm:"type:decimal(15,2)" json:"base_fee"`
	HasAreaBasedFee     bool             `gorm:"default:false" json:"has_area_based_fee"`
	PricePerSquareMeter *decimal.Decimal `gorm:"type:decimal(15,2)" json:"price_per_square_meter"`

	// Document requirements
	RequiredDocuments []RequiredDocument `gorm:"foreignKey:ApplicationCategoryID" json:"required_documents,omitempty"`

	// Relationships
	Applications []Application `gorm:"foreignKey:ApplicationCategoryID" json:"applications,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// RequiredDocument defines mandatory documents for application categories
type RequiredDocument struct {
	ID                    uuid.UUID        `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationCategoryID uuid.UUID        `gorm:"type:uuid;not null;index" json:"application_category_id"`
	DocumentCategory      DocumentCategory `gorm:"type:varchar(50);not null" json:"document_category"`
	DocumentName          string           `gorm:"not null" json:"document_name"`
	Description           string           `gorm:"type:text" json:"description"`
	IsMandatory           bool             `gorm:"default:true" json:"is_mandatory"`
	IsActive              bool             `gorm:"default:true" json:"is_active"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ApplicationReview tracks departmental reviews
type ApplicationReview struct {
	ID            uuid.UUID         `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID         `gorm:"type:uuid;not null;index" json:"application_id"`
	Department    Department        `gorm:"type:varchar(30);not null" json:"department"`
	ReviewerID    string            `gorm:"not null" json:"reviewer_id"`
	ReviewerName  string            `gorm:"not null" json:"reviewer_name"`
	Status        ApplicationStatus `gorm:"type:varchar(20);not null" json:"status"`
	Comments      *string           `gorm:"type:text" json:"comments"`
	ReviewDate    time.Time         `gorm:"not null" json:"review_date"`

	// Relationships
	Application Application `gorm:"foreignKey:ApplicationID;constraint:OnDelete:CASCADE" json:"application"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Comment struct {
	ID            uuid.UUID   `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID   `gorm:"type:uuid;not null;index" json:"application_id"`
	CommentType   CommentType `gorm:"type:varchar(30);not null" json:"comment_type"`
	Department    *Department `gorm:"type:varchar(30)" json:"department"`
	UserID        *string     `json:"user_id"`
	UserName      *string     `json:"user_name"`
	Subject       *string     `json:"subject"`
	Content       string      `gorm:"type:text;not null" json:"content"`
	IsInternal    bool        `gorm:"default:false" json:"is_internal"` // Internal vs client-visible
	IsResolved    bool        `gorm:"default:false" json:"is_resolved"`
	ParentID      *uuid.UUID  `gorm:"type:uuid;index" json:"parent_id"` // For threaded comments

	// Relationships
	Application Application `gorm:"foreignKey:ApplicationID;constraint:OnDelete:CASCADE" json:"application"`
	Parent      *Comment    `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Replies     []Comment   `gorm:"foreignKey:ParentID" json:"replies,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

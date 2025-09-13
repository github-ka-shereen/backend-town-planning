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

// Application represents a single submission to the town planning system.
type Application struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	PlanNumber  string    `gorm:"unique;not null;index" json:"plan_number"`
	ApplicantID uuid.UUID `gorm:"type:uuid;not null;index" json:"applicant_id"`

	// Property details
	PropertyType *PropertyType `gorm:"type:varchar(30)" json:"property_type"`
	StandID      *string       `gorm:"index" json:"stand_id"`

	// Planning details
	PlanArea *decimal.Decimal `gorm:"type:decimal(15,2)" json:"plan_area"`

	// Financial calculations
	PricePerSquareMeter *decimal.Decimal `gorm:"type:decimal(15,2)" json:"price_per_square_meter"`
	PermitFee           *decimal.Decimal `gorm:"type:decimal(15,2)" json:"permit_fee"`
	InspectionFee       *decimal.Decimal `gorm:"type:decimal(15,2)" json:"inspection_fee"`
	DevelopmentLevy     *decimal.Decimal `gorm:"type:decimal(15,2)" json:"development_levy"`
	VATAmount           *decimal.Decimal `gorm:"type:decimal(15,2)" json:"vat_amount"`
	TotalCost           *decimal.Decimal `gorm:"type:decimal(15,2)" json:"total_cost"`
	EstimatedCost       *decimal.Decimal `gorm:"type:decimal(15,2)" json:"estimated_cost"`

	// Payment tracking
	PaymentStatus PaymentStatus    `gorm:"type:varchar(20);default:'PENDING'" json:"payment_status"`
	ReceiptNumber *string          `gorm:"index" json:"receipt_number"`
	PaymentDate   *time.Time       `json:"payment_date"`
	AmountPaid    *decimal.Decimal `gorm:"type:decimal(15,2)" json:"amount_paid"`

	// Status and dates
	Status         ApplicationStatus `gorm:"type:varchar(20);default:'SUBMITTED';index" json:"status"`
	SubmissionDate time.Time         `gorm:"not null" json:"submission_date"`
	ApprovalDate   *time.Time        `json:"approval_date"`
	RejectionDate  *time.Time        `json:"rejection_date"`
	CollectionDate *time.Time        `json:"collection_date"`

	// Collection tracking
	IsCollected bool    `gorm:"default:false" json:"is_collected"`
	CollectedBy *string `json:"collected_by"`

	// Relationships - CORRECTED
	Applicant Applicant  `gorm:"foreignKey:ApplicantID" json:"applicant"`
	Documents []Document `gorm:"foreignKey:ApplicationID" json:"documents,omitempty"`
	Comments  []Comment  `gorm:"foreignKey:ApplicationID" json:"comments,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Comment struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"application_id"`
	Department    *string    `gorm:"type:varchar(30)" json:"department"` // Changed to string
	UserID        *uuid.UUID `gorm:"type:uuid;index" json:"user_id"`     // Changed to UUID
	Subject       *string    `json:"subject"`
	Content       string     `gorm:"type:text;not null" json:"content"`
	IsInternal    bool       `gorm:"default:false" json:"is_internal"`
	IsResolved    bool       `gorm:"default:false" json:"is_resolved"`
	IsActive      bool       `gorm:"default:true" json:"is_active"` // Changed default to true
	ParentID      *uuid.UUID `gorm:"type:uuid;index" json:"parent_id"`
	DocumentID    *uuid.UUID `gorm:"type:uuid;index" json:"document_id"`

	// Relationships - CORRECTED
	Application Application `gorm:"foreignKey:ApplicationID" json:"application"`
	Parent      *Comment    `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Replies     []Comment   `gorm:"foreignKey:ParentID" json:"replies,omitempty"` // Renamed for clarity
	User        User        `gorm:"foreignKey:UserID" json:"user"`
	Document    Document    `gorm:"foreignKey:DocumentID" json:"document"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

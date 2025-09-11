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
	Submitted  ApplicationStatus = "SUBMITTED"
	InProgress ApplicationStatus = "IN_PROGRESS"
	Approved   ApplicationStatus = "APPROVED"
	Rejected   ApplicationStatus = "REJECTED"
	Collected  ApplicationStatus = "COLLECTED"
)

// Application represents a single submission to the town planning system.
type Application struct {
	ID                    uuid.UUID           `gorm:"type:uuid;primary_key;" json:"id"`
	ClientID              uuid.UUID           `json:"client_id"`
	Client                Client              `gorm:"foreignKey:ClientID" json:"client"`
	ApplicationCategoryID uint                `json:"application_category_id"`
	ApplicationCategory   ApplicationCategory `gorm:"foreignKey:ApplicationCategoryID" json:"application_category"`
	ApplicationNumber     *string             `json:"application_number"`
	SubmissionDate        time.Time           `json:"submission_date"`
	Status                ApplicationStatus   `gorm:"type:varchar(20);default:'SUBMITTED'" json:"status"`
	CollectionStatus      *bool               `gorm:"default:false" json:"collection_status"`
	ClientDocuments       []ClientDocument    `gorm:"foreignKey:ApplicationID" json:"client_documents"`
	Comments              []Comment           `gorm:"foreignKey:ApplicationID" json:"comments"`

	// Financial and Calculation fields
	PlanArea            *decimal.Decimal `json:"plan_area"`
	PricePerSquareMeter *decimal.Decimal `json:"price_per_square_meter"`
	PermitFee           *decimal.Decimal `json:"permit_fee"`
	InspectionFee       *decimal.Decimal `json:"inspection_fee"`
	DevelopmentLevy     *decimal.Decimal `json:"development_levy"`
	VAT                 *decimal.Decimal `json:"vat"`
	TotalCost           *decimal.Decimal `json:"total_cost"`
	ReceiptNumber       *string          `json:"receipt_number"`

	// Metadata
	CreatedBy string         `json:"created_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Comment represents a comment from a specific department or user.
type Comment struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicationID uint           `json:"application_id"`
	UserID        string         `json:"user_id"` // Represents the staff member who made the comment
	Department    string         `gorm:"type:varchar(50)" json:"department"`
	Text          string         `gorm:"type:text" json:"text"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

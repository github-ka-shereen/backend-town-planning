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

type PaymentStatus string

const (
	PendingPayment   PaymentStatus = "PENDING"
	PaidPayment      PaymentStatus = "PAID"
	PartialPayment   PaymentStatus = "PARTIAL"
	RefundedPayment  PaymentStatus = "REFUNDED"
	CancelledPayment PaymentStatus = "CANCELLED"
)

// PropertyType model for dynamic property types
type PropertyType struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string    `gorm:"unique;not null" json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"` // System types cannot be modified
	IsActive    bool      `gorm:"default:true" json:"is_active"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	CreatedBy string         `gorm:"not null" json:"created_by"`
}

// Tariff model for dynamic pricing with validity period
type Tariff struct {
	ID              uuid.UUID       `gorm:"type:uuid;primary_key;" json:"id"`
	PropertyTypeID  uuid.UUID       `gorm:"type:uuid;not null;index" json:"property_type_id"`
	PricePerSqM     decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"price_per_sq_m"`
	PermitFee       decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"permit_fee"`
	InspectionFee   decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"inspection_fee"`
	DevelopmentLevy decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"development_levy"` // Store as percentage (e.g., 15.00 for 15%)
	ValidFrom       time.Time       `gorm:"not null;index" json:"valid_from"`
	ValidTo         *time.Time      `gorm:"index" json:"valid_to"` // NULL means currently active
	IsActive        bool            `gorm:"default:true" json:"is_active"`

	// Relationships
	PropertyType PropertyType `gorm:"foreignKey:PropertyTypeID" json:"property_type"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// VATRate model with validity period
type VATRate struct {
	ID        uuid.UUID       `gorm:"type:uuid;primary_key;" json:"id"`
	Rate      decimal.Decimal `gorm:"type:decimal(5,2);not null" json:"rate"` // Store as percentage (e.g., 15.00 for 15%)
	ValidFrom time.Time       `gorm:"not null;index" json:"valid_from"`
	ValidTo   *time.Time      `gorm:"index" json:"valid_to"` // NULL means currently active
	IsActive  bool            `gorm:"default:true" json:"is_active"`

	// Audit fields
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Application model
type Application struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	PlanNumber  string    `gorm:"unique;not null;index" json:"plan_number"`

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

	// Property details
	PropertyTypeID *uuid.UUID `gorm:"type:uuid;index" json:"property_type_id"`
	StandID        *string    `gorm:"index" json:"stand_id"`
	ApplicantID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"applicant_id"`

	// Reference to the actual rates used for this application
	TariffID  *uuid.UUID `gorm:"type:uuid;index" json:"tariff_id"`
	VATRateID *uuid.UUID `gorm:"type:uuid;index" json:"vat_rate_id"`

	// Relationships
	Applicant    Applicant     `gorm:"foreignKey:ApplicantID" json:"applicant"`
	PropertyType *PropertyType `gorm:"foreignKey:PropertyTypeID" json:"property_type"`
	Tariff       *Tariff       `gorm:"foreignKey:TariffID" json:"tariff,omitempty"`
	VATRate      *VATRate      `gorm:"foreignKey:VATRateID" json:"vat_rate,omitempty"`
	Documents    []Document    `gorm:"foreignKey:ApplicationID" json:"documents,omitempty"`
	Comments     []Comment     `gorm:"foreignKey:ApplicationID" json:"comments,omitempty"`

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

// PropertyType
func (pt *PropertyType) BeforeCreate(tx *gorm.DB) (err error) {
	if pt.ID == uuid.Nil {
		pt.ID = uuid.New()
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

// // SeedPropertyTypes populates the database with system property types
// func SeedPropertyTypes(db *gorm.DB, createdBy string) error {
// 	propertyTypes := []PropertyType{
// 		{
// 			Name:        "INDUSTRIAL",
// 			Description: "Industrial properties and facilities",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "COMMERCIAL",
// 			Description: "Commercial properties and business establishments",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "CHURCH",
// 			Description: "Religious and church properties",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "HIGH_DENSITY_RESIDENTIAL",
// 			Description: "High density residential properties (apartments, flats)",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "MEDIUM_DENSITY_RESIDENTIAL",
// 			Description: "Medium density residential properties (townhouses, duplexes)",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "LOW_DENSITY_RESIDENTIAL",
// 			Description: "Low density residential properties (single family homes)",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "HOLIDAY_HOME",
// 			Description: "Holiday and vacation homes",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "GOVERNMENT",
// 			Description: "Government properties and facilities",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "EDUCATIONAL",
// 			Description: "Educational institutions and facilities",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "HEALTHCARE",
// 			Description: "Healthcare facilities and medical centers",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 		{
// 			Name:        "RECREATIONAL",
// 			Description: "Recreational facilities and entertainment venues",
// 			IsSystem:    true,
// 			IsActive:    true,
// 			CreatedBy:   createdBy,
// 		},
// 	}

// 	for _, propertyType := range propertyTypes {
// 		// Check if property type already exists
// 		var existingPropertyType PropertyType
// 		if err := db.Where("name = ?", propertyType.Name).First(&existingPropertyType).Error; err != nil {
// 			if err == gorm.ErrRecordNotFound {
// 				// Property type doesn't exist, create it
// 				if err := db.Create(&propertyType).Error; err != nil {
// 					return err
// 				}
// 			} else {
// 				return err
// 			}
// 		} else {
// 			// Property type exists, update it if needed
// 			if existingPropertyType.IsSystem {
// 				// For system types, only update description and active status
// 				if err := db.Model(&existingPropertyType).Updates(map[string]interface{}{
// 					"description": propertyType.Description,
// 					"is_active":   propertyType.IsActive,
// 					"updated_at":  time.Now(),
// 				}).Error; err != nil {
// 					return err
// 				}
// 			} else {
// 				// For custom types, update all fields except IsSystem
// 				if err := db.Model(&existingPropertyType).Updates(map[string]interface{}{
// 					"description": propertyType.Description,
// 					"is_active":   propertyType.IsActive,
// 					"updated_at":  time.Now(),
// 				}).Error; err != nil {
// 					return err
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }
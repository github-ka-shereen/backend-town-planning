package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type StandCurrency string

const (
	USDStandCurrency StandCurrency = "USD"
	ZWLStandCurrency StandCurrency = "ZWL"
)

type Status string

const (
	UnallocatedStatus          Status = "unallocated"
	SwappedStatus              Status = "swapped"
	DonatedStatus              Status = "donated"
	ReservedStatus             Status = "reserved"
	ReservedWithoutOwnerStatus Status = "reserved_without_owner"
	FullyPaidStatus            Status = "fully_paid"
	PrePlanActivationStatus    Status = "pre_plan_activation"
	OngoingPaymentStatus       Status = "ongoing_payment"
)

// StandType represents the type of stand (e.g., Residential, Commercial, Industrial, etc.)
type StandType struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	Name        string         `gorm:"unique;not null" json:"name"`
	Description *string        `json:"description"`
	IsSystem    bool           `gorm:"default:false" json:"is_system"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedBy   string         `gorm:"not null" json:"created_by"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Project represents a development project that may contain multiple stands
type Project struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ProjectName   string    `gorm:"not null;index" json:"project_name"`
	Description   *string   `gorm:"type:text" json:"description"`
	ProjectNumber string    `gorm:"unique;not null;index" json:"project_number"`

	// Project Address and Location
	Address string `gorm:"not null" json:"address"`
	City    string `gorm:"not null" json:"city"`
	Country string `gorm:"default:'Zimbabwe'" json:"country"`

	// Geographic Information
	Latitude  *decimal.Decimal `gorm:"type:decimal(10,8)" json:"latitude"`
	Longitude *decimal.Decimal `gorm:"type:decimal(11,8)" json:"longitude"`

	// Project Details
	DeveloperID     *uuid.UUID `gorm:"type:uuid;index" json:"developer_id"`
	StartDate       *time.Time `json:"start_date"`
	EndDate         *time.Time `json:"end_date"`
	TotalStands     int        `gorm:"default:0" json:"total_stands"`
	StandsSold      int        `gorm:"default:0" json:"stands_sold"`
	StandsAvailable int        `gorm:"default:0" json:"stands_available"`

	// Relationships - FIXED: Now Document has ProjectID field
	Developer *Applicant `gorm:"foreignKey:DeveloperID" json:"developer,omitempty"`
	Stands    []Stand    `gorm:"foreignKey:ProjectID" json:"stands,omitempty"`
	Documents []Document `gorm:"foreignKey:ProjectID" json:"documents,omitempty"`

	// Audit fields
	CreatedBy string         `gorm:"not null" json:"created_by"`
	UpdatedBy *string        `json:"updated_by"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Stand represents a plot of land or property unit
type Stand struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	StandNumber   string    `gorm:"unique;not null;index" json:"stand_number"`
	AccountNumber *string   `json:"account_number"`

	// Project Reference (Optional)
	ProjectID *uuid.UUID `gorm:"type:uuid;index" json:"project_id"`

	// Location Information (Used when Project is not available)
	Address       *string `json:"address"` // Optional if project has address
	StreetName    *string `json:"street_name"`
	Suburb        *string `json:"suburb"`
	TownCity      *string `json:"town_city"`
	ProvinceState *string `json:"province_state"`
	PostalCode    *string `json:"postal_code"`
	Country       string  `gorm:"default:'Zimbabwe'" json:"country"`

	// Geographic Information
	Latitude        *decimal.Decimal `gorm:"type:decimal(10,8)" json:"latitude"`
	Longitude       *decimal.Decimal `gorm:"type:decimal(11,8)" json:"longitude"`
	AreaSquareMeter *decimal.Decimal `gorm:"type:decimal(15,2)" json:"area_square_meter"`
	AreaHectare     *decimal.Decimal `gorm:"type:decimal(15,4)" json:"area_hectare"`

	// Stand Classification
	StandTypeID    *uuid.UUID `gorm:"type:uuid;index" json:"stand_type_id"`
	StandSizeID    *uuid.UUID `gorm:"type:uuid;index" json:"stand_size_id"`
	PropertyTypeID *uuid.UUID `gorm:"type:uuid;index" json:"property_type_id"`
	ZoneCategory   *string    `gorm:"type:varchar(50);index" json:"zone_category"`

	// Ownership Information
	CurrentOwnerID   *uuid.UUID `gorm:"type:uuid;index" json:"current_owner_id"`
	PreviousOwnerID  *uuid.UUID `gorm:"type:uuid;index" json:"previous_owner_id"`
	OwnershipType    *string    `gorm:"type:varchar(30);index" json:"ownership_type"`
	RegistrationDate *time.Time `json:"registration_date"`

	// Survey and Deeds Information
	DeedsNumber    *string    `gorm:"index" json:"deeds_number"`
	SurveyorName   *string    `json:"surveyor_name"`
	SurveyDate     *time.Time `json:"survey_date"`
	BeaconDetails  *string    `gorm:"type:text" json:"beacon_details"`
	DiagramNumber  *string    `gorm:"index" json:"diagram_number"`
	GeneralPlanRef *string    `gorm:"index" json:"general_plan_ref"`

	// Status and Flags
	IsOccupied    bool `gorm:"default:false" json:"is_occupied"`
	IsVacant      bool `gorm:"default:false" json:"is_vacant"`
	HasStructures bool `gorm:"default:false" json:"has_structures"`
	IsServiced    bool `gorm:"default:false" json:"is_serviced"`
	IsApproved    bool `gorm:"default:false" json:"is_approved"`
	IsActive      bool `gorm:"default:true" json:"is_active"`

	// This is the cost of the stand *before* VAT
	TaxExclusiveStandPrice decimal.Decimal `gorm:"type:decimal(18,8)" json:"tax_exclusive_stand_price"`

	// This is the total cost of the stand *including* VAT
	StandCost       decimal.Decimal `gorm:"type:decimal(18,8)" json:"stand_cost"`              // This field now includes VAT
	VATAmount       decimal.Decimal `gorm:"type:decimal(18,8);default:0.00" json:"vat_amount"` // The 15% VAT charged on the TaxExclusiveStandPrice
	TitleDeedNumber *string         `json:"title_deed_number"`
	Status          Status          `json:"status"`

	// Relationships
	Project        *Project          `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	StandType      *StandType        `gorm:"foreignKey:StandTypeID" json:"stand_type,omitempty"`
	StandSize      decimal.Decimal   `gorm:"type:decimal(18,8)" json:"stand_size"`
	StandCurrency  StandCurrency     `json:"stand_currency"`
	PropertyType   *PropertyType     `gorm:"foreignKey:PropertyTypeID" json:"property_type,omitempty"`
	CurrentOwner   *Applicant        `gorm:"foreignKey:CurrentOwnerID" json:"current_owner,omitempty"`
	PreviousOwner  *Applicant        `gorm:"foreignKey:PreviousOwnerID" json:"previous_owner,omitempty"`
	AllStandOwners *[]AllStandOwners `gorm:"foreignKey:StandID;references:ID" json:"all_stand_owners"`
	Applications   []Application     `gorm:"foreignKey:StandID" json:"applications,omitempty"`
	Documents      []Document        `gorm:"foreignKey:StandID" json:"documents,omitempty"` // Survey Diagram, General Plan, etc.

	// Audit fields
	CreatedBy  string         `gorm:"not null" json:"created_by"`
	UpdatedBy  *string        `json:"updated_by"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	ReservedAt *time.Time     `gorm:"column:reserved_at" json:"reserved_at"`
	DonatedAt  *time.Time     `gorm:"column:donated_at" json:"donated_at"`
	SoldAt     *time.Time     `json:"sold_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

type AllStandOwners struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	StandID       uuid.UUID  `json:"stand_id"`
	PaymentPlanID *uuid.UUID `gorm:"type:uuid;null;index" json:"payment_plan_id"`
	ApplicantID   uuid.UUID  `gorm:"index:idx_applicant_id_is_liaison" json:"applicant_id"`
	IsLiaison     bool       `gorm:"index" json:"is_liaison"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	// Add this field to create the relationship with Client
	Applicant *Applicant     `gorm:"foreignKey:ApplicantID;references:ID" json:"applicant"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate hooks
func (st *StandType) BeforeCreate(tx *gorm.DB) error {
	if st.ID == uuid.Nil {
		st.ID = uuid.New()
	}
	return nil
}

func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (s *Stand) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

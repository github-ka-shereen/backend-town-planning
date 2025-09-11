package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ClientType string

const (
	Individual ClientType = "INDIVIDUAL"
	Company    ClientType = "COMPANY"
	// Add other types as needed
)

// ClientStatus defines the current status of a client within the system.
type ClientStatus string

const (
	Prospective ClientStatus = "PROSPECTIVE"
	Active      ClientStatus = "ACTIVE"
	Inactive    ClientStatus = "INACTIVE"
)

// Client represents the core entity applying for services.
type Client struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	ClientType  ClientType `json:"client_type"` // INDIVIDUAL, COMPANY, etc.
	FirstName   *string    `json:"first_name"`  // Optional for company clients
	LastName    *string    `json:"last_name"`   // Optional for company clients
	MiddleName  *string    `json:"middle_name"`
	DateOfBirth *time.Time `json:"date_of_birth"`
	Gender      *string    `json:"gender"`
	Occupation  *string    `json:"occupation"`

	// Company specific fields
	CompanyName             *string `json:"company_name"`              // Required for COMPANY type
	TaxIdentificationNumber *string `json:"tax_identification_number"` // Company tax ID

	// Relationships
	CompanyRepresentatives       []CompanyRepresentative        `gorm:"many2many:client_company_representatives;" json:"company_representatives"`
	Applications                 []Application                  `gorm:"foreignKey:ClientID" json:"applications"`
	ClientAdditionalPhoneNumbers []ClientAdditionalPhoneNumbers `gorm:"foreignKey:ClientID" json:"client_additional_phone_numbers"`

	// Contact information
	PostalAddress *string      `json:"postal_address"`
	City          *string      `json:"city"`
	Email         string       `json:"email"`                       // Primary contact email
	IdNumber      *string      `json:"id_number"`                   // National Registration Card
	PhoneNumber   string       `json:"phone_number"`                // Primary contact number
	Status        ClientStatus `json:"status"`                      // PROSPECTIVE/ACTIVE/INACTIVE
	FullName      string       `json:"full_name"`                   // Computed field
	Debtor        bool         `gorm:"default:false" json:"debtor"` // Owes money to council

	// Metadata
	CreatedBy string    `json:"created_by"` // Staff ID who created record
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ClientDocument tracks supporting documents submitted with applications
type ClientDocument struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	ClientID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"client_id"`
	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id"` // Optional link to specific application
	DocumentID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	CreatedBy     string     `json:"created_by"`

	Document    Document     `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
	Application *Application `gorm:"foreignKey:ApplicationID;constraint:OnDelete:SET NULL" json:"-"`
}

// ApplicationCategory represents a user-configurable application category within the town planning system.
type ApplicationCategory struct {
	Name        string         `gorm:"unique;not null" json:"name"`
	Description string         `json:"description"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy   string         `gorm:"not null" json:"created_by"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// ClientAdditionalPhoneNumbers stores alternate contact numbers
type ClientAdditionalPhoneNumbers struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ClientID    uuid.UUID `json:"client_id"`    // Parent client
	PhoneNumber string    `json:"phone_number"` // Additional contact
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy   string    `json:"created_by"` // Staff who added
}

// CompanyRepresentative identifies people authorized for company applications
type CompanyRepresentative struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	Email       string    `json:"email"`
	PhoneNumber string    `json:"phone_number"`
	Role        string    `json:"role"` // Position in company
	Clients     []Client  `gorm:"many2many:client_company_representatives;" json:"clients"`

	CreatedBy string    `gorm:"not null" json:"created_by"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ClientCompanyRepresentative join table for company reps
type ClientCompanyRepresentative struct {
	ClientID                uuid.UUID `gorm:"primaryKey"`
	CompanyRepresentativeID uuid.UUID `gorm:"primaryKey"`
	CreatedAt               time.Time `gorm:"autoCreateTime"`
}

// TableName overrides default table name for join table
func (ClientCompanyRepresentative) TableName() string {
	return "client_company_representatives"
}

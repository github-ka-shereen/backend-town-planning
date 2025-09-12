package models

import (
	"time"

	"github.com/google/uuid"
)

type ApplicantType string

const (
	IndividualApplicant   ApplicantType = "INDIVIDUAL"
	OrganisationApplicant ApplicantType = "ORGANISATION"
)

type ApplicantStatus string

const (
	ProspectiveApplicant ApplicantStatus = "PROSPECTIVE"
	ActiveApplicant      ApplicantStatus = "ACTIVE"
	InactiveApplicant    ApplicantStatus = "INACTIVE"
	BlacklistedApplicant ApplicantStatus = "BLACKLISTED"
)

// Applicant represents the core entity applying for services.
type Applicant struct {
	ID            uuid.UUID     `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicantType ApplicantType `json:"applicant_type"` // INDIVIDUAL, ORGANISATION, etc.
	FirstName     *string       `json:"first_name"`     // Optional for organisation applicants
	LastName      *string       `json:"last_name"`      // Optional for organisation applicants
	MiddleName    *string       `json:"middle_name"`
	DateOfBirth   *time.Time    `json:"date_of_birth"`
	Gender        *string       `json:"gender"`
	Occupation    *string       `json:"occupation"`

	// Organisation specific fields
	OrganisationName        *string `json:"organisation_name"`         // Required for ORGANISATION type
	TaxIdentificationNumber *string `json:"tax_identification_number"` // Organisation tax ID

	// Relationships
	OrganisationRepresentatives     []OrganisationRepresentative      `gorm:"many2many:applicant_organisation_representatives;" json:"organisation_representatives"`
	Applications                    []Application                     `gorm:"foreignKey:ApplicantID" json:"applications"`
	ApplicantAdditionalPhoneNumbers []ApplicantAdditionalPhoneNumbers `gorm:"foreignKey:ApplicantID" json:"applicant_additional_phone_numbers"`

	// Contact information
	PostalAddress  *string         `json:"postal_address"`
	City           *string         `json:"city"`
	WhatsAppNumber *string         `json:"whatsapp_number"`
	Email          string          `json:"email"`                       // Primary contact email
	IdNumber       *string         `json:"id_number"`                   // National Registration Card
	PhoneNumber    string          `json:"phone_number"`                // Primary contact number
	Status         ApplicantStatus `json:"status"`                      // PROSPECTIVE/ACTIVE/INACTIVE
	FullName       string          `json:"full_name"`                   // Computed field
	Debtor         bool            `gorm:"default:false" json:"debtor"` // Owes money to council

	// Metadata
	CreatedBy string    `json:"created_by"` // Staff ID who created record
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ApplicantDocument tracks supporting documents submitted with applications
type ApplicantDocument struct {
	ID            uuid.UUID    `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicantID   uuid.UUID    `gorm:"type:uuid;not null;index" json:"applicant_id"`
	ApplicationID *uuid.UUID   `gorm:"type:uuid;index" json:"application_id"` // Optional link to specific application
	DocumentID    uuid.UUID    `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedAt     time.Time    `gorm:"autoCreateTime" json:"created_at"`
	CreatedBy     string       `json:"created_by"`
	Document      Document     `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
	Application   *Application `gorm:"foreignKey:ApplicationID;constraint:OnDelete:SET NULL" json:"application"`
}

// ApplicantAdditionalPhoneNumbers stores alternate contact numbers
type ApplicantAdditionalPhoneNumbers struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicantID uuid.UUID `json:"applicant_id"` // Parent applicant
	PhoneNumber string    `json:"phone_number"` // Additional contact
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy   string    `json:"created_by"` // Staff who added
}

// OrganisationRepresentative identifies people authorized for organisation applications
type OrganisationRepresentative struct {
	ID             uuid.UUID   `gorm:"type:uuid;primary_key;" json:"id"`
	FirstName      string      `json:"first_name"`
	LastName       string      `json:"last_name"`
	Email          string      `json:"email"`
	PhoneNumber    string      `json:"phone_number"`
	WhatsAppNumber *string     `json:"whatsapp_number"`
	Role           string      `json:"role"` // Position in organisation
	Applicants     []Applicant `gorm:"many2many:applicant_organisation_representatives;" json:"applicants"`

	CreatedBy string    `gorm:"not null" json:"created_by"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ApplicantOrganisationRepresentative join table for organisation reps
type ApplicantOrganisationRepresentative struct {
	ApplicantID                  uuid.UUID `gorm:"primaryKey"`
	OrganisationRepresentativeID uuid.UUID `gorm:"primaryKey"`
	CreatedAt                    time.Time `gorm:"autoCreateTime"`
}

// TableName overrides default table name for join table
func (ApplicantOrganisationRepresentative) TableName() string {
	return "applicant_organisation_representatives"
}

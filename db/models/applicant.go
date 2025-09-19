package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	ApplicantType ApplicantType `json:"applicant_type"`
	FirstName     *string       `json:"first_name"`
	LastName      *string       `json:"last_name"`
	MiddleName    *string       `json:"middle_name"`
	DateOfBirth   *time.Time    `json:"date_of_birth"`
	Gender        *string       `json:"gender"`
	Occupation    *string       `json:"occupation"`

	// Organisation specific fields
	OrganisationName        *string `json:"organisation_name"`
	TaxIdentificationNumber *string `json:"tax_identification_number"`

	// Relationships - CORRECTED
	OrganisationRepresentatives []OrganisationRepresentative `gorm:"many2many:applicant_organisation_representatives;foreignKey:ID;joinForeignKey:ApplicantID;References:ID;joinReferences:OrganisationRepresentativeID" json:"organisation_representatives"`
	Applications                []Application                `gorm:"foreignKey:ApplicantID" json:"applications"`
	AdditionalPhoneNumbers      []ApplicantAdditionalPhone   `gorm:"foreignKey:ApplicantID" json:"additional_phone_numbers"` // Renamed for consistency

	// Contact information
	PostalAddress  *string         `json:"postal_address"`
	City           *string         `json:"city"`
	WhatsAppNumber *string         `json:"whatsapp_number"`
	Email          string          `json:"email"`
	IdNumber       *string         `json:"id_number"`
	PhoneNumber    string          `json:"phone_number"`
	Status         ApplicantStatus `json:"status"`
	FullName       string          `json:"full_name" gorm:"-"`
	Debtor         bool            `gorm:"default:false" json:"debtor"`

	// Metadata
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ApplicantDocument tracks supporting documents submitted with applications
type ApplicantDocument struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicantID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"applicant_id"`
	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id"`
	DocumentID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	CreatedBy     string     `json:"created_by"`

	// Relationships - CORRECTED
	Document    Document     `gorm:"foreignKey:DocumentID" json:"document"`
	Application *Application `gorm:"foreignKey:ApplicationID" json:"application"`
	Applicant   Applicant    `gorm:"foreignKey:ApplicantID" json:"applicant"`
}

// ApplicantAdditionalPhone stores alternate contact numbers
type ApplicantAdditionalPhone struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ApplicantID uuid.UUID `gorm:"type:uuid;not null;index" json:"applicant_id"`
	PhoneNumber string    `json:"phone_number"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy   string    `json:"created_by"`

	// Relationship - ADDED
	Applicant Applicant `gorm:"foreignKey:ApplicantID" json:"applicant"`
}

// OrganisationRepresentative identifies people authorized for organisation applications
type OrganisationRepresentative struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Email          string    `json:"email"`
	PhoneNumber    string    `json:"phone_number"`
	WhatsAppNumber *string   `json:"whatsapp_number"`
	Role           string    `json:"role"`

	// Relationships - CORRECTED
	Applicants []Applicant `gorm:"many2many:applicant_organisation_representatives;foreignKey:ID;joinForeignKey:OrganisationRepresentativeID;References:ID;joinReferences:ApplicantID" json:"applicants"`

	CreatedBy string    `gorm:"not null" json:"created_by"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ApplicantOrganisationRepresentative join table for organisation reps
type ApplicantOrganisationRepresentative struct {
	ApplicantID                  uuid.UUID `gorm:"type:uuid;primaryKey" json:"applicant_id"`
	OrganisationRepresentativeID uuid.UUID `gorm:"type:uuid;primaryKey" json:"organisation_representative_id"`
	CreatedAt                    time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships - ADDED for better querying
	Applicant                  Applicant                  `gorm:"foreignKey:ApplicantID" json:"applicant"`
	OrganisationRepresentative OrganisationRepresentative `gorm:"foreignKey:OrganisationRepresentativeID" json:"organisation_representative"`
}

// TableName overrides default table name for join table
func (ApplicantOrganisationRepresentative) TableName() string {
	return "applicant_organisation_representatives"
}

func (a *Applicant) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return
}

func (ad *ApplicantDocument) BeforeCreate(tx *gorm.DB) (err error) {
	if ad.ID == uuid.Nil {
		ad.ID = uuid.New()
	}
	return
}

func (ap *ApplicantAdditionalPhone) BeforeCreate(tx *gorm.DB) (err error) {
	if ap.ID == uuid.Nil {
		ap.ID = uuid.New()
	}
	return
}

func (or *OrganisationRepresentative) BeforeCreate(tx *gorm.DB) (err error) {
	if or.ID == uuid.Nil {
		or.ID = uuid.New()
	}
	return
}

// Helper method to get display name
func (c *Applicant) GetFullName() string {
	if c.ApplicantType == OrganisationApplicant && c.OrganisationName != nil {
		return *c.OrganisationName
	}
	return *c.FirstName + " " + *c.LastName
}

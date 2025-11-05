package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type EmailLog struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	Recipient      string    `gorm:"not null" json:"recipient"`
	Subject        string    `gorm:"not null" json:"subject"`
	Message        string    `gorm:"type:text;not null" json:"message"`
	SentAt         time.Time `gorm:"not null" json:"sent_at"`
	Active         *bool     `gorm:"default:true" json:"active"`
	AttachmentPath string    `json:"attachment_path"` // Legacy field for backward compatibility

	// NEW: Document relationships using join tables
	EmailDocuments []EmailDocument `gorm:"foreignKey:EmailLogID" json:"email_documents,omitempty"`

	// Relationships to other entities (optional)
	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id,omitempty"`
	ApplicantID   *uuid.UUID `gorm:"type:uuid;index" json:"applicant_id,omitempty"`
	PaymentID     *uuid.UUID `gorm:"type:uuid;index" json:"payment_id,omitempty"`

	// Relationships
	Application *Application `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Applicant   *Applicant   `gorm:"foreignKey:ApplicantID" json:"applicant,omitempty"`
	Payment     *Payment     `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`

	// Additional email metadata
	EmailType    string  `gorm:"type:varchar(50)" json:"email_type"` // e.g., "APPLICATION_SUBMITTED", "PAYMENT_RECEIPT"
	Status       string  `gorm:"type:varchar(20);default:'SENT'" json:"status"` // SENT, FAILED, DELIVERED
	Error        *string `gorm:"type:text" json:"error,omitempty"`
	TemplateName *string `gorm:"type:varchar(100)" json:"template_name,omitempty"`

	// Audit fields
	CreatedBy string    `gorm:"not null" json:"created_by"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// EmailDocument represents the relationship between email logs and documents
type EmailDocument struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	EmailLogID uuid.UUID `gorm:"type:uuid;not null;index" json:"email_log_id"`
	DocumentID uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	EmailLog EmailLog `gorm:"foreignKey:EmailLogID;constraint:OnDelete:CASCADE" json:"email_log"`
	Document Document `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
}

// BeforeCreate hooks
func (e *EmailLog) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.SentAt.IsZero() {
		e.SentAt = time.Now()
	}
	return nil
}

func (ed *EmailDocument) BeforeCreate(tx *gorm.DB) error {
	if ed.ID == uuid.Nil {
		ed.ID = uuid.New()
	}
	return nil
}
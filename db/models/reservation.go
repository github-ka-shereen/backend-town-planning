package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReservationStatus string

const (
	ActiveReservationStatus    ReservationStatus = "active"
	ExpiredReservationStatus   ReservationStatus = "expired"
	CancelledReservationStatus ReservationStatus = "cancelled"
)

type Reservation struct {
	ID               uuid.UUID         `gorm:"type:uuid;primary_key;" json:"id"`
	StandID          uuid.UUID         `gorm:"type:uuid;not null;index" json:"stand_id"`
	Stand            Stand             `gorm:"foreignKey:StandID;references:ID" json:"stand"`
	ApplicantID      *uuid.UUID        `gorm:"type:uuid;null" json:"applicant_id"`
	Applicant        *Applicant        `gorm:"foreignKey:ApplicantID;references:ID" json:"applicant"`
	ReservationDate  time.Time         `gorm:"not null" json:"reservation_date"`
	ExpiryDate       *time.Time        `json:"expiry_date"`
	Status           ReservationStatus `gorm:"not null;default:'pending'" json:"status"`
	Comment          *string           `gorm:"type:text" json:"comment"`
	ReminderDate     *time.Time        `json:"reminder_date"`
	ReminderNote     *string           `gorm:"type:text" json:"reminder_note"`
	IsReminderActive bool              `gorm:"default:false" json:"is_reminder_active"`
	CreatedBy        string            `gorm:"not null" json:"created_by"`
	UpdatedBy        *string           `json:"updated_by"`
	CreatedAt        time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt    `gorm:"index" json:"-"`
}

func (r *Reservation) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

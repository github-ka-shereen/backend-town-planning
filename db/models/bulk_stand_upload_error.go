package models

import (
	"time"

	"github.com/google/uuid"
)

// BulkUploadError represents an error encountered during a bulk upload.
type BulkStandUploadError struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	StandNumber  string    `json:"stand_number"`  // The stand number that caused the error
	ErrorMessage string    `json:"error_message"` // The error message
	RowIndex     int       `json:"row_index"`     // The row index of the error in the bulk upload
	Resolved     bool      `json:"resolved"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy    string    `gorm:"not null" json:"created_by"` // Staff member adding the record
}

package models

import (
	"time"

	"github.com/google/uuid"
)

// BulkUploadErrorType represents the type of error (either duplicate or missing data)
type BulkUploadErrorType string

const (
	DuplicateErrorType   BulkUploadErrorType = "Duplicate"
	MissingDataErrorType BulkUploadErrorType = "Missing Data"
	CalculationErrorType BulkUploadErrorType = "CALCULATION_ERROR"
	NotFoundErrorType    BulkUploadErrorType = "Stand Not Found"
	UpdateErrorType      BulkUploadErrorType = "Failed to update stand"
)

type AddedViaType string

const (
	SingleAddedViaType AddedViaType = "Single"
	BulkAddedViaType   AddedViaType = "Bulk"
)

// BulkUploadError represents an error during bulk upload, either due to duplicates or missing data.
type BulkUploadErrorProjects struct {
	ID            uuid.UUID           `gorm:"type:uuid;primary_key;" json:"id"`
	ProjectNumber string              `json:"project_number"`
	ProjectName   string              `json:"project_name"`
	City          string              `json:"city"`
	Reason        string              `json:"reason"`
	Address       string              `json:"address"`
	CreatedBy     string              `json:"created_by"`
	ErrorType     BulkUploadErrorType `json:"error_type"` // New field for error type
	AddedVia      AddedViaType        `json:"added_via"`  // was it a single addition by user or bulk upload
	CreatedAt     time.Time           `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time           `gorm:"autoUpdateTime" json:"updated_at"`
}

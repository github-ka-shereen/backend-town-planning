package requests

import (
	"github.com/google/uuid"
)

type CreateDocumentRequest struct {
	FileName       string     `json:"file_name"`
	FileSize       string     `json:"file_size"`
	CategoryCode   string     `json:"category_code"`
	DocumentCategoryId *uuid.UUID `json:"document_category_id"`
	CreatedBy      string     `json:"created_by"`
	FileType       string     `json:"file_type"`

	// Entity relationships - support for all 9 join table entities
	ApplicantID   *uuid.UUID `json:"applicant_id,omitempty"`
	ApplicationID *uuid.UUID `json:"application_id,omitempty"`
	StandID       *uuid.UUID `json:"stand_id,omitempty"`
	ProjectID     *uuid.UUID `json:"project_id,omitempty"`
	PaymentID     *uuid.UUID `json:"payment_id,omitempty"`
	CommentID     *uuid.UUID `json:"comment_id,omitempty"`
	EmailLogID    *uuid.UUID `json:"email_log_id,omitempty"`
	BankID        *uuid.UUID `json:"bank_id,omitempty"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`

	// Legacy fields (consider deprecating)
	PlanID               *uuid.UUID `json:"plan_id,omitempty"`
	DevelopmentExpenseID *uuid.UUID `json:"development_expense_id,omitempty"`
	VendorID             *uuid.UUID `json:"vendor_id,omitempty"`
}

type LinkDocumentRequest struct {
	DocumentID    uuid.UUID  `json:"document_id"`
	CreatedBy     string     `json:"created_by"`

	// Entity relationships - support for all 9 join table entities
	ApplicantID   *uuid.UUID `json:"applicant_id,omitempty"`
	ApplicationID *uuid.UUID `json:"application_id,omitempty"`
	StandID       *uuid.UUID `json:"stand_id,omitempty"`
	ProjectID     *uuid.UUID `json:"project_id,omitempty"`
	PaymentID     *uuid.UUID `json:"payment_id,omitempty"`
	CommentID     *uuid.UUID `json:"comment_id,omitempty"`
	EmailLogID    *uuid.UUID `json:"email_log_id,omitempty"`
	BankID        *uuid.UUID `json:"bank_id,omitempty"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`

	// Legacy fields (consider deprecating)
	VendorID             *uuid.UUID `json:"vendor_id,omitempty"`
	DevelopmentExpenseID *uuid.UUID `json:"development_expense_id,omitempty"`
	PaymentPlanID        *uuid.UUID `json:"payment_plan_id,omitempty"`
}

type GetDocumentsByEntityRequest struct {
	EntityType string    `json:"entity_type"` 
	// Supported entity types: 
	// "applicant", "application", "stand", "project", 
	// "payment", "comment", "email", "bank", "user"
	EntityID   uuid.UUID `json:"entity_id"`
}

type CreateDocumentVersionRequest struct {
	OriginalDocumentID uuid.UUID `json:"original_document_id"`
	FileName           string    `json:"file_name"`
	FileSize           string    `json:"file_size"`
	FileType           string    `json:"file_type"`
	UpdateReason       string    `json:"update_reason"`
	CreatedBy          string    `json:"created_by"`
}

type BulkLinkDocumentsRequest struct {
	DocumentIDs []uuid.UUID `json:"document_ids"`
	LinkDocumentRequest
}

type UpdateDocumentRequest struct {
	DocumentID   uuid.UUID  `json:"document_id"`
	Description  *string    `json:"description,omitempty"`
	IsPublic     *bool      `json:"is_public,omitempty"`
	IsMandatory  *bool      `json:"is_mandatory,omitempty"`
	IsActive     *bool      `json:"is_active,omitempty"`
	CategoryID   *uuid.UUID `json:"category_id,omitempty"`
	UpdatedBy    string     `json:"updated_by"`
	UpdateReason *string    `json:"update_reason,omitempty"`
}

// NEW: Request for searching/filtering documents
type SearchDocumentsRequest struct {
	CategoryCode *string    `json:"category_code,omitempty"`
	DocumentType *string    `json:"document_type,omitempty"`
	FileName     *string    `json:"file_name,omitempty"`
	IsPublic     *bool      `json:"is_public,omitempty"`
	IsMandatory  *bool      `json:"is_mandatory,omitempty"`
	IsActive     *bool      `json:"is_active,omitempty"`
	CreatedBy    *string    `json:"created_by,omitempty"`
	ApplicantID  *uuid.UUID `json:"applicant_id,omitempty"`
	DateFrom     *string    `json:"date_from,omitempty"`
	DateTo       *string    `json:"date_to,omitempty"`
	Limit        int        `json:"limit" validate:"min=1,max=100"`
	Offset       int        `json:"offset" validate:"min=0"`
}

// NEW: Request for bulk document creation
type BulkCreateDocumentsRequest struct {
	Documents []BulkDocumentItem `json:"documents"`
}

type BulkDocumentItem struct {
	FileName     string     `json:"file_name"`
	FileSize     string     `json:"file_size"`
	CategoryCode string     `json:"category_code"`
	CreatedBy    string     `json:"created_by"`
	FileType     string     `json:"file_type"`
	ApplicantID  *uuid.UUID `json:"applicant_id,omitempty"`
	// Add other entity fields as needed
}

// NEW: Request for document statistics
type DocumentStatsRequest struct {
	EntityType  *string    `json:"entity_type,omitempty"`
	EntityID    *uuid.UUID `json:"entity_id,omitempty"`
	CategoryCode *string   `json:"category_code,omitempty"`
	DateFrom    *string    `json:"date_from,omitempty"`
	DateTo      *string    `json:"date_to,omitempty"`
}

// NEW: Request for document download with options
type DownloadDocumentRequest struct {
	DocumentID uuid.UUID `json:"document_id"`
	// Optional: Specify if you want a specific version
	Version    *int      `json:"version,omitempty"`
	// Optional: Watermark or other processing options
	Watermark  *bool     `json:"watermark,omitempty"`
}

// NEW: Request for document sharing
type ShareDocumentRequest struct {
	DocumentID   uuid.UUID `json:"document_id"`
	ShareWith    []string  `json:"share_with"` // Emails or user IDs
	AccessLevel  string    `json:"access_level" validate:"oneof=view download"`
	ExpiresAt    *string   `json:"expires_at,omitempty"` // ISO date string
	Message      *string   `json:"message,omitempty"`
	SharedBy     string    `json:"shared_by"`
}

// NEW: Request for document access control
type UpdateDocumentAccessRequest struct {
	DocumentID   uuid.UUID `json:"document_id"`
	UserID       uuid.UUID `json:"user_id"`
	AccessLevel  string    `json:"access_level" validate:"oneof=view download none"`
	UpdatedBy    string    `json:"updated_by"`
}

// NEW: Request for document categorization
type CategorizeDocumentsRequest struct {
	DocumentIDs  []uuid.UUID `json:"document_ids"`
	CategoryCode string      `json:"category_code"`
	UpdatedBy    string      `json:"updated_by"`
}

// NEW: Request for document archiving
type ArchiveDocumentsRequest struct {
	DocumentIDs []uuid.UUID `json:"document_ids"`
	ArchiveReason *string   `json:"archive_reason,omitempty"`
	ArchivedBy  string      `json:"archived_by"`
}

// Response structures (for completeness)
type DocumentResponse struct {
	ID           uuid.UUID `json:"id"`
	FileName     string    `json:"file_name"`
	FileSize     string    `json:"file_size"`
	DocumentType string    `json:"document_type"`
	CategoryCode string    `json:"category_code"`
	CreatedAt    string    `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
	IsPublic     bool      `json:"is_public"`
	IsMandatory  bool      `json:"is_mandatory"`
	Version      int       `json:"version"`
	IsCurrentVersion bool  `json:"is_current_version"`
	// Entity relationships
	LinkedEntities []LinkedEntity `json:"linked_entities,omitempty"`
}

type LinkedEntity struct {
	EntityType string    `json:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id"`
	EntityName string    `json:"entity_name,omitempty"`
}

type DocumentStatsResponse struct {
	TotalDocuments   int64 `json:"total_documents"`
	TotalSizeBytes   int64 `json:"total_size_bytes"`
	ByCategory       map[string]int64 `json:"by_category"`
	ByType           map[string]int64 `json:"by_type"`
	RecentUploads    int64 `json:"recent_uploads"` // Last 7 days
}

type BulkOperationResponse struct {
	SuccessCount int              `json:"success_count"`
	ErrorCount   int              `json:"error_count"`
	Errors       []OperationError `json:"errors,omitempty"`
}

type OperationError struct {
	DocumentID *uuid.UUID `json:"document_id,omitempty"`
	Error      string     `json:"error"`
}
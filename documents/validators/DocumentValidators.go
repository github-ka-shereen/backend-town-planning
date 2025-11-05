package validators

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"town-planning-backend/db/models"

	documents_requests "town-planning-backend/documents/requests"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type DocumentValidator struct{}

func NewDocumentValidator() *DocumentValidator {
	return &DocumentValidator{}
}

// ValidateCreateDocumentRequest validates the incoming document creation request
func (v *DocumentValidator) ValidateCreateDocumentRequest(req *documents_requests.CreateDocumentRequest) error {
	if err := v.validateFileName(req.FileName); err != nil {
		return err
	}

	// File size validation commented out as per your service logic
	// if err := v.validateFileSize(req.FileSize); err != nil {
	// 	return err
	// }

	if req.CategoryCode == "" {
		return errors.New("category_code cannot be empty")
	}

	if err := v.validateFileType(req.FileType); err != nil {
		return err
	}

	if err := v.validateCreatedBy(req.CreatedBy); err != nil {
		return err
	}

	if err := v.validateRelationships(req); err != nil {
		return err
	}

	return nil
}

// validateFileName ensures the filename is valid
func (v *DocumentValidator) validateFileName(fileName string) error {
	if strings.TrimSpace(fileName) == "" {
		return errors.New("file name cannot be empty")
	}

	if len(fileName) > 255 {
		return errors.New("file name cannot exceed 255 characters")
	}

	return nil
}

// validateFileSize ensures the file size is valid
func (v *DocumentValidator) validateFileSize(fileSize string) error {
	if strings.TrimSpace(fileSize) == "" {
		return errors.New("file size cannot be empty")
	}

	size, err := decimal.NewFromString(fileSize)
	if err != nil {
		return fmt.Errorf("invalid file size format: %v", err)
	}

	if size.IsNegative() {
		return errors.New("file size cannot be negative")
	}

	// Set maximum file size (e.g., 100MB)
	maxSize := decimal.NewFromInt(100 * 1024 * 1024)
	if size.GreaterThan(maxSize) {
		return errors.New("file size exceeds maximum allowed size (100MB)")
	}

	return nil
}

// validateFileType ensures the file type is supported
func (v *DocumentValidator) validateFileType(fileType string) error {
	allowedMimeTypes := map[string]bool{
		"application/pdf":    true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"text/plain":      true,
		"image/jpeg":      true,
		"image/png":       true,
		"application/rtf": true,
		"application/vnd.oasis.opendocument.text": true,
		"image/gif":     true,
		"image/svg+xml": true,
		"image/webp":    true,
		"image/bmp":     true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
		"application/vnd.ms-excel": true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		"application/vnd.ms-powerpoint":                                             true,
	}

	cleanFileType := strings.TrimSpace(strings.ToLower(fileType))
	if !allowedMimeTypes[cleanFileType] {
		return fmt.Errorf("invalid file type: %s", fileType)
	}

	return nil
}

// validateCreatedBy ensures the created_by field is valid
func (v *DocumentValidator) validateCreatedBy(createdBy string) error {
	if strings.TrimSpace(createdBy) == "" {
		return errors.New("created_by cannot be empty")
	}

	if len(createdBy) > 100 {
		return errors.New("created_by cannot exceed 100 characters")
	}

	return nil
}

// validateRelationships ensures that the document is properly associated with at least one entity
func (v *DocumentValidator) validateRelationships(req *documents_requests.CreateDocumentRequest) error {
	hasRelationship := false

	// Check all 9 supported entity types
	if req.ApplicantID != nil {
		hasRelationship = true
	}

	if req.ApplicationID != nil {
		hasRelationship = true
	}

	if req.StandID != nil {
		hasRelationship = true
	}

	if req.ProjectID != nil {
		hasRelationship = true
	}

	if req.PaymentID != nil {
		hasRelationship = true
	}

	if req.CommentID != nil {
		hasRelationship = true
	}

	if req.EmailLogID != nil {
		hasRelationship = true
	}

	if req.BankID != nil {
		hasRelationship = true
	}

	if req.UserID != nil {
		hasRelationship = true
	}

	// Legacy entity checks (for backward compatibility)
	if req.VendorID != nil {
		hasRelationship = true
	}

	if req.DevelopmentExpenseID != nil {
		hasRelationship = true
	}

	if !hasRelationship {
		return errors.New("document must be associated with at least one entity (applicant, application, stand, project, payment, comment, email, bank, or user)")
	}

	return nil
}

// ValidateLinkDocumentRequest validates the link document request
func (v *DocumentValidator) ValidateLinkDocumentRequest(req *documents_requests.LinkDocumentRequest) error {
	if req.DocumentID == uuid.Nil {
		return errors.New("document_id cannot be empty")
	}

	if err := v.validateCreatedBy(req.CreatedBy); err != nil {
		return err
	}

	// Validate that at least one entity is specified
	hasEntity := req.ApplicantID != nil ||
		req.ApplicationID != nil ||
		req.StandID != nil ||
		req.ProjectID != nil ||
		req.PaymentID != nil ||
		req.CommentID != nil ||
		req.EmailLogID != nil ||
		req.BankID != nil ||
		req.UserID != nil

	if !hasEntity {
		return errors.New("must specify at least one entity to link with")
	}

	return nil
}

// ValidateGetDocumentsByEntityRequest validates the get documents by entity request
func (v *DocumentValidator) ValidateGetDocumentsByEntityRequest(req *documents_requests.GetDocumentsByEntityRequest) error {
	validEntityTypes := map[string]bool{
		"applicant":   true,
		"application": true,
		"stand":       true,
		"project":     true,
		"payment":     true,
		"comment":     true,
		"email":       true,
		"bank":        true,
		"user":        true,
	}

	if !validEntityTypes[req.EntityType] {
		return fmt.Errorf("invalid entity type: %s. Valid types are: applicant, application, stand, project, payment, comment, email, bank, user", req.EntityType)
	}

	if req.EntityID == uuid.Nil {
		return errors.New("entity_id cannot be empty")
	}

	return nil
}

// ValidateUpdateDocumentRequest validates the update document request
func (v *DocumentValidator) ValidateUpdateDocumentRequest(req *documents_requests.UpdateDocumentRequest) error {
	if req.DocumentID == uuid.Nil {
		return errors.New("document_id cannot be empty")
	}

	if err := v.validateCreatedBy(req.UpdatedBy); err != nil {
		return err
	}

	// Validate that at least one field is being updated
	if req.Description == nil && req.IsPublic == nil && req.IsMandatory == nil &&
		req.IsActive == nil && req.CategoryID == nil {
		return errors.New("at least one field must be provided for update")
	}

	return nil
}

// ValidateSearchDocumentsRequest validates the search documents request
func (v *DocumentValidator) ValidateSearchDocumentsRequest(req *documents_requests.SearchDocumentsRequest) error {
	if req.Limit < 1 || req.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}

	if req.Offset < 0 {
		return errors.New("offset cannot be negative")
	}

	// Validate date range if provided
	if req.DateFrom != nil || req.DateTo != nil {
		if err := v.validateDateRange(req.DateFrom, req.DateTo); err != nil {
			return err
		}
	}

	return nil
}

// validateDateRange validates date range parameters
func (v *DocumentValidator) validateDateRange(dateFrom, dateTo *string) error {
	// Simple date format validation (YYYY-MM-DD)
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	if dateFrom != nil && !dateRegex.MatchString(*dateFrom) {
		return errors.New("date_from must be in YYYY-MM-DD format")
	}

	if dateTo != nil && !dateRegex.MatchString(*dateTo) {
		return errors.New("date_to must be in YYYY-MM-DD format")
	}

	return nil
}

// ValidateUUID validates if a string is a valid UUID
func (v *DocumentValidator) ValidateUUID(id string) (*uuid.UUID, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID format: %v", err)
	}

	return &parsedID, nil
}

// GetDocumentType returns the document type based on extension
func (v *DocumentValidator) GetDocumentType(ext string) (models.DocumentType, error) {
	switch strings.ToLower(ext) {
	case ".doc", ".docx":
		return models.WordDocumentType, nil
	case ".rtf", ".odt", ".txt":
		return models.TextDocumentType, nil
	case ".xls", ".xlsx", ".csv":
		return models.SpreadsheetType, nil
	case ".ppt", ".pptx":
		return models.PresentationType, nil
	case ".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".bmp", ".tiff", ".tif":
		return models.ImageType, nil
	case ".pdf":
		return models.PDFType, nil
	case ".dwg", ".dxf":
		return models.CADDrawingType, nil
	case ".zip", ".rar", ".7z":
		// For survey plans and other compressed documents
		return models.SurveyPlanType, nil
	default:
		return "", fmt.Errorf("unrecognized file extension: %s", ext)
	}
}

// ValidateDocumentType validates if a document type is supported
func (v *DocumentValidator) ValidateDocumentType(docType string) error {
	validTypes := map[string]bool{
		string(models.WordDocumentType):       true,
		string(models.TextDocumentType):       true,
		string(models.SpreadsheetType):        true,
		string(models.PresentationType):       true,
		string(models.ImageType):              true,
		string(models.PDFType):                true,
		string(models.CADDrawingType):         true,
		string(models.SurveyPlanType):         true,
		string(models.EngineeringCertificate): true,
		string(models.BuildingPlanType):       true,
		string(models.SitePlanType):           true,
	}

	if !validTypes[docType] {
		return fmt.Errorf("invalid document type: %s", docType)
	}

	return nil
}

// SanitizeFileName cleans and formats the filename
func (v *DocumentValidator) SanitizeFileName(name string) string {
	// Remove invalid characters
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	cleaned := reg.ReplaceAllString(name, "")

	// Replace spaces with underscores
	cleaned = strings.ReplaceAll(cleaned, " ", "_")

	// Remove consecutive underscores
	cleaned = strings.ReplaceAll(cleaned, "__", "_")

	// Remove leading/trailing underscores and dots
	cleaned = strings.Trim(cleaned, "_. ")

	// Limit length
	if len(cleaned) > 240 { // Leave room for extension
		cleaned = cleaned[:240]
	}

	return cleaned
}

// ValidateCategoryCode validates category code format
func (v *DocumentValidator) ValidateCategoryCode(categoryCode string) error {
	if strings.TrimSpace(categoryCode) == "" {
		return errors.New("category code cannot be empty")
	}

	// Category code should be alphanumeric with underscores
	match, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", categoryCode)
	if !match {
		return errors.New("category code can only contain letters, numbers, and underscores")
	}

	if len(categoryCode) > 50 {
		return errors.New("category code cannot exceed 50 characters")
	}

	return nil
}

// ValidateBulkOperation validates bulk operation requests
func (v *DocumentValidator) ValidateBulkOperation(documentIDs []uuid.UUID) error {
	if len(documentIDs) == 0 {
		return errors.New("no document IDs provided")
	}

	if len(documentIDs) > 100 {
		return errors.New("cannot process more than 100 documents in a single operation")
	}

	// Check for duplicate IDs
	seen := make(map[uuid.UUID]bool)
	for _, id := range documentIDs {
		if seen[id] {
			return fmt.Errorf("duplicate document ID: %s", id.String())
		}
		seen[id] = true
	}

	return nil
}

// ValidateShareDocumentRequest validates document sharing request
func (v *DocumentValidator) ValidateShareDocumentRequest(req *documents_requests.ShareDocumentRequest) error {
	if req.DocumentID == uuid.Nil {
		return errors.New("document_id cannot be empty")
	}

	if len(req.ShareWith) == 0 {
		return errors.New("must specify at least one recipient")
	}

	if len(req.ShareWith) > 50 {
		return errors.New("cannot share with more than 50 recipients at once")
	}

	// Validate email format for recipients
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	for _, recipient := range req.ShareWith {
		if !emailRegex.MatchString(recipient) {
			if _, err := uuid.Parse(recipient); err != nil {
				return fmt.Errorf("invalid recipient format: %s. Must be a valid email or UUID", recipient)
			}
		}
	}

	if err := v.validateCreatedBy(req.SharedBy); err != nil {
		return err
	}

	return nil
}

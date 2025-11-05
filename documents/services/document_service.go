package services

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/documents/repositories"
	documents_requests "town-planning-backend/documents/requests"
	"town-planning-backend/documents/validators"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// VersionInfo holds versioning data
type VersionInfo struct {
	Version    int
	IsCurrent  bool
	PreviousID *uuid.UUID
	OriginalID *uuid.UUID
}

type DocumentService struct {
	Validator    *validators.DocumentValidator
	DocumentRepo repositories.DocumentRepository
	FileStorage  utils.FileStorage
}

type CreateDocumentResponse struct {
	ID       uuid.UUID        `json:"id"`
	Document *models.Document `json:"document"`
}

func NewDocumentService(repo repositories.DocumentRepository, fileStorage utils.FileStorage) *DocumentService {
	return &DocumentService{
		Validator:    validators.NewDocumentValidator(),
		DocumentRepo: repo,
		FileStorage:  fileStorage,
	}
}

func (s *DocumentService) UnifiedCreateDocument(
	tx *gorm.DB,
	c *fiber.Ctx,
	request *documents_requests.CreateDocumentRequest,
	fileContent []byte,
	fileHeader *multipart.FileHeader,
) (*CreateDocumentResponse, error) {

	config.Logger.Info("Unified document creation started",
		zap.String("category_code", request.CategoryCode),
		zap.Any("applicant_id", request.ApplicantID))

	// Validate request
	if err := s.Validator.ValidateCreateDocumentRequest(request); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Look up category
	category, err := s.DocumentRepo.GetCategoryByCode(tx, request.CategoryCode)
	if err != nil {
		return nil, fmt.Errorf("category lookup failed: %w", err)
	}

	// Get applicant for folder structure
	var applicant *models.Applicant
	if request.ApplicantID != nil {
		applicant, err = s.DocumentRepo.GetApplicant(tx, *request.ApplicantID)
		if err != nil {
			config.Logger.Warn("Applicant not found, using general folder", zap.Error(err))
		}
	}

	// Handle file upload
	var filePath, fileName string
	var fileSize int64

	if fileHeader != nil {
		filePath, fileName, fileSize, err = s.saveMultipartFile(fileHeader, request, applicant)
	} else if fileContent != nil {
		if len(fileContent) == 0 {
			config.Logger.Warn("File content is empty but proceeding", zap.String("filename", request.FileName))
			// Don't return error for empty files, just log warning
		}
		filePath, fileName, fileSize, err = s.saveByteFile(fileContent, request, applicant)
	} else {
		return nil, fmt.Errorf("no file content provided")
	}

	if err != nil {
		return nil, fmt.Errorf("file save failed: %w", err)
	}

	// Validate computed file size - allow zero-sized files but log warning
	if fileSize < 0 {
		s.cleanupFile(filePath)
		return nil, fmt.Errorf("invalid file size: %d bytes", fileSize)
	}

	if fileSize == 0 {
		config.Logger.Warn("File has zero size",
			zap.String("filename", fileName),
			zap.String("path", filePath))
	}

	config.Logger.Info("File processed successfully",
		zap.String("path", filePath),
		zap.String("name", fileName),
		zap.Int64("size_bytes", fileSize))

	// Handle versioning
	versionInfo, err := s.prepareVersioning(tx, request, category.ID)
	if err != nil {
		s.cleanupFile(filePath)
		return nil, fmt.Errorf("versioning preparation failed: %w", err)
	}

	// Create document record with computed file size
	document, err := s.createDocumentRecord(request, category.ID, fileName, filePath, fileSize, versionInfo)
	if err != nil {
		s.cleanupFile(filePath)
		return nil, err
	}

	// Verify document has valid file size before saving (allow zero but not negative)
	if document.FileSize.LessThan(decimal.NewFromInt(0)) {
		s.cleanupFile(filePath)
		return nil, fmt.Errorf("document file size is negative")
	}

	config.Logger.Info("Document record created",
		zap.String("doc_id", document.ID.String()),
		zap.String("file_size", document.FileSize.String()))

	// Save with audit trail
	createdDocument, err := s.saveDocumentWithAudit(tx, c, document, request.CreatedBy)
	if err != nil {
		s.cleanupFile(filePath)
		return nil, fmt.Errorf("document save failed: %w", err)
	}

	// Create entity-document relationships based on request
	if err := s.createEntityDocumentRelationships(tx, request, createdDocument.ID); err != nil {
		config.Logger.Error("Failed to create entity-document relationships", zap.Error(err))
		// Don't return error here as the document was created successfully
	}

	config.Logger.Info("Document created successfully",
		zap.String("document_id", createdDocument.ID.String()),
		zap.String("file_path", filePath),
		zap.Int64("file_size", fileSize))

	return &CreateDocumentResponse{
		ID:       createdDocument.ID,
		Document: createdDocument,
	}, nil
}

// Create entity-document relationships based on request
func (s *DocumentService) createEntityDocumentRelationships(
	tx *gorm.DB,
	request *documents_requests.CreateDocumentRequest,
	documentID uuid.UUID,
) error {
	var relationships []interface{}

	// Create relationships based on what's provided in the request
	if request.ApplicantID != nil {
		relationships = append(relationships, &models.ApplicantDocument{
			ID:            uuid.New(),
			ApplicantID:   *request.ApplicantID,
			DocumentID:    documentID,
			ApplicationID: request.ApplicationID,
			CreatedBy:     request.CreatedBy,
		})
	}

	if request.ApplicationID != nil {
		relationships = append(relationships, &models.ApplicationDocument{
			ID:            uuid.New(),
			ApplicationID: *request.ApplicationID,
			DocumentID:    documentID,
			CreatedBy:     request.CreatedBy,
		})
	}

	if request.StandID != nil {
		relationships = append(relationships, &models.StandDocument{
			ID:         uuid.New(),
			StandID:    *request.StandID,
			DocumentID: documentID,
			CreatedBy:  request.CreatedBy,
		})
	}

	if request.ProjectID != nil {
		relationships = append(relationships, &models.ProjectDocument{
			ID:         uuid.New(),
			ProjectID:  *request.ProjectID,
			DocumentID: documentID,
			CreatedBy:  request.CreatedBy,
		})
	}

	if request.PaymentID != nil {
		relationships = append(relationships, &models.PaymentDocument{
			ID:         uuid.New(),
			PaymentID:  *request.PaymentID,
			DocumentID: documentID,
			CreatedBy:  request.CreatedBy,
		})
	}

	if request.CommentID != nil {
		relationships = append(relationships, &models.CommentDocument{
			ID:         uuid.New(),
			CommentID:  *request.CommentID,
			DocumentID: documentID,
			CreatedBy:  request.CreatedBy,
		})
	}

	if request.EmailLogID != nil {
		relationships = append(relationships, &models.EmailDocument{
			ID:         uuid.New(),
			EmailLogID: *request.EmailLogID,
			DocumentID: documentID,
			CreatedBy:  request.CreatedBy,
		})
	}

	if request.BankID != nil {
		relationships = append(relationships, &models.BankDocument{
			ID:         uuid.New(),
			BankID:     *request.BankID,
			DocumentID: documentID,
			CreatedBy:  request.CreatedBy,
		})
	}

	if request.UserID != nil {
		relationships = append(relationships, &models.UserDocument{
			ID:         uuid.New(),
			UserID:     *request.UserID,
			DocumentID: documentID,
			CreatedBy:  request.CreatedBy,
		})
	}

	// Create all relationships in transaction
	for _, rel := range relationships {
		if err := tx.Create(rel).Error; err != nil {
			config.Logger.Warn("Failed to create document relationship, continuing",
				zap.Error(err),
				zap.String("document_id", documentID.String()))
			// Continue with other relationships instead of failing
		}
	}

	return nil
}

// CreateCouncilApplicantDocuments - Updated for Applicant model
func (s *DocumentService) CreateCouncilApplicantDocuments(
	tx *gorm.DB,
	c *fiber.Ctx,
	applicantID uuid.UUID,
	files []*multipart.FileHeader,
	metadataList []*documents_requests.CreateDocumentRequest,
) ([]*models.Document, error) {

	if len(files) != len(metadataList) {
		return nil, fmt.Errorf("files/metadata count mismatch: %d files, %d metadata", len(files), len(metadataList))
	}

	var createdDocuments []*models.Document

	for i, fileHeader := range files {
		meta := metadataList[i]
		meta.ApplicantID = &applicantID

		response, err := s.UnifiedCreateDocument(tx, c, meta, nil, fileHeader)
		if err != nil {
			config.Logger.Error("Failed to process document, skipping",
				zap.Int("index", i+1),
				zap.String("filename", fileHeader.Filename),
				zap.Error(err))
			continue // Skip this document but continue with others
		}

		createdDocuments = append(createdDocuments, response.Document)
	}

	if len(createdDocuments) == 0 {
		return nil, fmt.Errorf("no documents were successfully processed")
	}

	return createdDocuments, nil
}

// File handling methods
func (s *DocumentService) saveMultipartFile(
	fileHeader *multipart.FileHeader,
	request *documents_requests.CreateDocumentRequest,
	applicant *models.Applicant,
) (string, string, int64, error) {

	src, err := fileHeader.Open()
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	return s.saveFileStream(src, fileHeader.Filename, fileHeader.Size, request, applicant)
}

func (s *DocumentService) saveByteFile(
	fileContent []byte,
	request *documents_requests.CreateDocumentRequest,
	applicant *models.Applicant,
) (string, string, int64, error) {

	fileSize := int64(len(fileContent))

	config.Logger.Info("Processing byte file",
		zap.String("filename", request.FileName),
		zap.Int64("size_bytes", fileSize))

	reader := bytes.NewReader(fileContent)

	filePath, fileName, _, err := s.saveFileStream(reader, request.FileName, fileSize, request, applicant)
	if err != nil {
		return "", "", 0, err
	}

	return filePath, fileName, fileSize, nil
}

func (s *DocumentService) saveFileStream(
	src io.Reader,
	originalName string,
	fileSize int64,
	request *documents_requests.CreateDocumentRequest,
	applicant *models.Applicant,
) (string, string, int64, error) {

	folderPath, fileName := s.generateOrganizedFileStructure(request, applicant, originalName)
	fullPath := filepath.Join(folderPath, fileName)

	if err := s.ensureDirectoryExists(folderPath); err != nil {
		return "", "", 0, fmt.Errorf("failed to create directory: %w", err)
	}

	filePath, err := s.FileStorage.UploadFileFromReader(src, fullPath)
	if err != nil {
		return "", "", 0, fmt.Errorf("file storage failed: %w", err)
	}

	return filePath, fileName, fileSize, nil
}

func (s *DocumentService) ensureDirectoryExists(dirPath string) error {
	baseDir := "uploads"
	fullPath := filepath.Join(baseDir, dirPath)

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
	}

	config.Logger.Debug("Directory created successfully",
		zap.String("path", fullPath))

	return nil
}

func (s *DocumentService) generateOrganizedFileStructure(
	request *documents_requests.CreateDocumentRequest,
	applicant *models.Applicant,
	originalName string,
) (string, string) {

	fileExt := strings.ToLower(filepath.Ext(originalName))
	if fileExt == "" {
		fileExt = ".dat"
	}

	folderPath := s.generateFolderPath(applicant, request.CategoryCode)
	fileName := s.generateDescriptiveFilename(request, applicant, fileExt)

	return folderPath, fileName
}

func (s *DocumentService) generateFolderPath(applicant *models.Applicant, categoryCode string) string {
	if applicant != nil {
		return filepath.Join("applicants", applicant.ID.String(), categoryCode)
	}
	return filepath.Join("general", categoryCode)
}

func (s *DocumentService) generateDescriptiveFilename(
	request *documents_requests.CreateDocumentRequest,
	applicant *models.Applicant,
	fileExt string,
) string {

	timestamp := time.Now().Format("20060102_150405")
	shortUUID := uuid.New().String()[:8]

	var applicantName string
	if applicant != nil {
		applicantName = s.sanitizeForFilename(applicant.GetFullName())
	} else {
		applicantName = "unknown"
	}

	filename := fmt.Sprintf("%s_%s_%s_v1_%s%s",
		request.CategoryCode,
		applicantName,
		timestamp,
		shortUUID,
		fileExt,
	)

	return filename
}

func (s *DocumentService) prepareVersioning(
	tx *gorm.DB,
	request *documents_requests.CreateDocumentRequest,
	categoryID uuid.UUID,
) (*VersionInfo, error) {

	entityType, entityID := s.determineEntityType(request)

	config.Logger.Info("🔍 Versioning check starting",
		zap.String("entityType", entityType),
		zap.Any("entityID", entityID),
		zap.String("categoryCode", request.CategoryCode),
		zap.String("categoryID", categoryID.String()))

	// Call FindExistingDocument
	existingDoc, err := s.DocumentRepo.FindExistingDocument(tx, categoryID, entityType, entityID)

	// Handle errors from FindExistingDocument
	if err != nil {
		config.Logger.Error("❌ Versioning check failed with error",
			zap.Error(err),
			zap.String("entityType", entityType),
			zap.Any("entityID", entityID))
		return nil, fmt.Errorf("versioning check failed: %w", err)
	}

	// No existing document found - this is the normal case for new entities
	if existingDoc == nil {
		config.Logger.Info("✅ No existing document found - starting with version 1",
			zap.String("entityType", entityType),
			zap.Any("entityID", entityID),
			zap.String("category", request.CategoryCode))

		return &VersionInfo{
			Version:    1,
			IsCurrent:  true,
			PreviousID: nil,
			OriginalID: nil,
		}, nil
	}

	// Existing document found - create new version
	config.Logger.Info("📄 Found existing document for versioning",
		zap.String("existingDocID", existingDoc.ID.String()),
		zap.Int("existingVersion", existingDoc.Version),
		zap.String("entityType", entityType),
		zap.Any("entityID", entityID))

	// Archive the existing version
	if err := s.archiveDocumentVersion(tx, existingDoc); err != nil {
		config.Logger.Error("Failed to archive previous version",
			zap.Error(err),
			zap.String("documentID", existingDoc.ID.String()))
		return nil, fmt.Errorf("failed to archive previous version: %w", err)
	}

	// Determine original ID for the version chain
	originalID := existingDoc.OriginalID
	if originalID == nil {
		// If existing doc has no original ID, it IS the original
		originalID = &existingDoc.ID
	}

	config.Logger.Info("✅ Version prepared successfully",
		zap.Int("newVersion", existingDoc.Version+1),
		zap.String("previousID", existingDoc.ID.String()),
		zap.String("originalID", originalID.String()))

	return &VersionInfo{
		Version:    existingDoc.Version + 1,
		IsCurrent:  true,
		PreviousID: &existingDoc.ID,
		OriginalID: originalID,
	}, nil
}

func (s *DocumentService) determineEntityType(request *documents_requests.CreateDocumentRequest) (string, *uuid.UUID) {
	switch {
	case request.ApplicationID != nil:
		return "application", request.ApplicationID
	case request.StandID != nil:
		return "stand", request.StandID
	case request.ApplicantID != nil:
		return "applicant", request.ApplicantID
	case request.ProjectID != nil:
		return "project", request.ProjectID
	case request.PaymentID != nil:
		return "payment", request.PaymentID
	case request.CommentID != nil:
		return "comment", request.CommentID
	case request.EmailLogID != nil:
		return "email", request.EmailLogID
	case request.BankID != nil:
		return "bank", request.BankID
	case request.UserID != nil:
		return "user", request.UserID
	default:
		return "general", nil
	}
}

func (s *DocumentService) archiveDocumentVersion(tx *gorm.DB, doc *models.Document) error {
	doc.IsCurrentVersion = false
	return tx.Save(doc).Error
}

// Document record creation
func (s *DocumentService) createDocumentRecord(
	request *documents_requests.CreateDocumentRequest,
	categoryID uuid.UUID,
	fileName, filePath string,
	fileSize int64,
	versionInfo *VersionInfo,
) (*models.Document, error) {

	fileExt := strings.ToLower(filepath.Ext(fileName))
	documentType, err := s.Validator.GetDocumentType(fileExt)
	if err != nil {
		return nil, fmt.Errorf("invalid file type: %w", err)
	}

	document := &models.Document{
		ID:               uuid.New(),
		FileName:         fileName,
		DocumentType:     documentType,
		FileSize:         decimal.NewFromInt(fileSize),
		CategoryID:       &categoryID,
		CreatedBy:        request.CreatedBy,
		FilePath:         filePath,
		FileHash:         s.calculateFileHash(fileName, fileSize),
		MimeType:         s.getMimeType(documentType),
		Description:      &request.FileName,
		IsPublic:         false,
		IsMandatory:      true,
		IsActive:         true,
		Version:          versionInfo.Version,
		IsCurrentVersion: versionInfo.IsCurrent,
		PreviousID:       versionInfo.PreviousID,
	}

	if versionInfo.OriginalID != nil {
		document.OriginalID = versionInfo.OriginalID
	} else {
		document.OriginalID = &document.ID
	}

	return document, nil
}

// Helper methods
func (s *DocumentService) saveDocumentWithAudit(
	tx *gorm.DB,
	c *fiber.Ctx,
	document *models.Document,
	createdBy string,
) (*models.Document, error) {

	if c != nil {
		return s.DocumentRepo.CreateDocumentWithAudit(
			tx, document, createdBy, createdBy, "user", c.IP(), c.Get("User-Agent"),
		)
	}
	return s.DocumentRepo.CreateDocument(tx, document)
}

func (s *DocumentService) cleanupFile(filePath string) {
	if err := s.FileStorage.DeleteFile(filePath); err != nil {
		config.Logger.Error("Failed to cleanup file", zap.Error(err), zap.String("file_path", filePath))
	}
}

func (s *DocumentService) sanitizeForFilename(name string) string {
	sanitized := strings.ReplaceAll(name, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, "..", "")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, "*", "-")
	sanitized = strings.ReplaceAll(sanitized, "?", "-")
	sanitized = strings.ReplaceAll(sanitized, "\"", "-")
	sanitized = strings.ReplaceAll(sanitized, "<", "-")
	sanitized = strings.ReplaceAll(sanitized, ">", "-")
	sanitized = strings.ReplaceAll(sanitized, "|", "-")
	return sanitized
}

func (s *DocumentService) calculateFileHash(fileName string, fileSize int64) string {
	return fmt.Sprintf("%s-%d", fileName, fileSize)
}

func (s *DocumentService) getMimeType(docType models.DocumentType) string {
	mimeTypes := map[models.DocumentType]string{
		models.WordDocumentType:       "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		models.TextDocumentType:       "text/plain",
		models.SpreadsheetType:        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		models.PresentationType:       "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		models.ImageType:              "image/jpeg",
		models.PDFType:                "application/pdf",
		models.CADDrawingType:         "application/dwg",
		models.SurveyPlanType:         "application/pdf",
		models.EngineeringCertificate: "application/pdf",
		models.BuildingPlanType:       "application/pdf",
		models.SitePlanType:           "application/pdf",
	}
	if mime, ok := mimeTypes[docType]; ok {
		return mime
	}
	return "application/octet-stream"
}

// Method to link existing document to entities
func (s *DocumentService) LinkDocumentToEntities(
	tx *gorm.DB,
	documentID uuid.UUID,
	request *documents_requests.LinkDocumentRequest,
) error {

	linkRequest := &documents_requests.CreateDocumentRequest{
		ApplicantID:   request.ApplicantID,
		ApplicationID: request.ApplicationID,
		StandID:       request.StandID,
		ProjectID:     request.ProjectID,
		PaymentID:     request.PaymentID,
		CommentID:     request.CommentID,
		EmailLogID:    request.EmailLogID,
		BankID:        request.BankID,
		UserID:        request.UserID,
		CreatedBy:     request.CreatedBy,
	}

	return s.createEntityDocumentRelationships(tx, linkRequest, documentID)
}

// Method to get documents by entity
func (s *DocumentService) GetDocumentsByEntity(
	tx *gorm.DB,
	entityType string,
	entityID uuid.UUID,
) ([]*models.Document, error) {

	switch entityType {
	case "applicant":
		var applicantDocs []models.ApplicantDocument
		if err := tx.Preload("Document").Where("applicant_id = ?", entityID).Find(&applicantDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(applicantDocs))
		for i, ad := range applicantDocs {
			documents[i] = &ad.Document
		}
		return documents, nil

	case "application":
		var appDocs []models.ApplicationDocument
		if err := tx.Preload("Document").Where("application_id = ?", entityID).Find(&appDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(appDocs))
		for i, ad := range appDocs {
			documents[i] = &ad.Document
		}
		return documents, nil

	case "stand":
		var standDocs []models.StandDocument
		if err := tx.Preload("Document").Where("stand_id = ?", entityID).Find(&standDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(standDocs))
		for i, sd := range standDocs {
			documents[i] = &sd.Document
		}
		return documents, nil

	case "project":
		var projectDocs []models.ProjectDocument
		if err := tx.Preload("Document").Where("project_id = ?", entityID).Find(&projectDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(projectDocs))
		for i, pd := range projectDocs {
			documents[i] = &pd.Document
		}
		return documents, nil

	case "payment":
		var paymentDocs []models.PaymentDocument
		if err := tx.Preload("Document").Where("payment_id = ?", entityID).Find(&paymentDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(paymentDocs))
		for i, pd := range paymentDocs {
			documents[i] = &pd.Document
		}
		return documents, nil

	case "comment":
		var commentDocs []models.CommentDocument
		if err := tx.Preload("Document").Where("comment_id = ?", entityID).Find(&commentDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(commentDocs))
		for i, cd := range commentDocs {
			documents[i] = &cd.Document
		}
		return documents, nil

	case "email":
		var emailDocs []models.EmailDocument
		if err := tx.Preload("Document").Where("email_log_id = ?", entityID).Find(&emailDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(emailDocs))
		for i, ed := range emailDocs {
			documents[i] = &ed.Document
		}
		return documents, nil

	case "bank":
		var bankDocs []models.BankDocument
		if err := tx.Preload("Document").Where("bank_id = ?", entityID).Find(&bankDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(bankDocs))
		for i, bd := range bankDocs {
			documents[i] = &bd.Document
		}
		return documents, nil

	case "user":
		var userDocs []models.UserDocument
		if err := tx.Preload("Document").Where("user_id = ?", entityID).Find(&userDocs).Error; err != nil {
			return nil, err
		}
		documents := make([]*models.Document, len(userDocs))
		for i, ud := range userDocs {
			documents[i] = &ud.Document
		}
		return documents, nil

	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// New method to get all documents with their relationships
func (s *DocumentService) GetDocumentWithRelationships(
	tx *gorm.DB,
	documentID uuid.UUID,
) (*models.Document, error) {

	var document models.Document
	err := tx.
		Preload("ApplicantDocuments").
		Preload("ApplicationDocuments").
		Preload("StandDocuments").
		Preload("ProjectDocuments").
		Preload("CommentDocuments").
		Preload("PaymentDocuments").
		Preload("EmailDocuments").
		Preload("BankDocuments").
		Preload("UserDocuments").
		First(&document, "id = ?", documentID).Error

	if err != nil {
		return nil, err
	}

	return &document, nil
}

// Method to delete document and all its relationships
func (s *DocumentService) DeleteDocumentWithRelationships(
	tx *gorm.DB,
	documentID uuid.UUID,
) error {

	// Get document first to get file path
	var document models.Document
	if err := tx.First(&document, "id = ?", documentID).Error; err != nil {
		return err
	}

	// Delete all relationships first
	if err := tx.Where("document_id = ?", documentID).Delete(&models.ApplicantDocument{}).Error; err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", documentID).Delete(&models.ApplicationDocument{}).Error; err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", documentID).Delete(&models.StandDocument{}).Error; err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", documentID).Delete(&models.ProjectDocument{}).Error; err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", documentID).Delete(&models.CommentDocument{}).Error; err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", documentID).Delete(&models.PaymentDocument{}).Error; err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", documentID).Delete(&models.EmailDocument{}).Error; err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", documentID).Delete(&models.BankDocument{}).Error; err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", documentID).Delete(&models.UserDocument{}).Error; err != nil {
		return err
	}

	// Delete the document
	if err := tx.Delete(&document).Error; err != nil {
		return err
	}

	// Delete the physical file
	s.cleanupFile(document.FilePath)

	return nil
}

package services

// import (
// 	"bytes"
// 	"town-planning-backend/config"
// 	"town-planning-backend/db/models"
// 	"town-planning-backend/documents/repositories"
// 	"town-planning-backend/documents/validators"
// 	"town-planning-backend/utils"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"mime/multipart"
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"time"

// 	documents_requests "town-planning-backend/documents/requests"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/google/uuid"
// 	"github.com/shopspring/decimal"
// 	"gorm.io/gorm"

// 	"go.uber.org/zap"
// )

// type DocumentService struct {
// 	Validator    *validators.DocumentValidator
// 	DocumentRepo repositories.DocumentRepository
// 	FileStorage  utils.FileStorage
// }

// type CreateDocumentResponse struct {
// 	ID          uuid.UUID        `json:"id"`
// 	Document    *models.Document `json:"document"`
// 	DownloadURL string           `json:"download_url"`
// }

// func NewDocumentService(repo repositories.DocumentRepository, fileStorage utils.FileStorage) *DocumentService {
// 	return &DocumentService{
// 		Validator:    validators.NewDocumentValidator(),
// 		DocumentRepo: repo,
// 		FileStorage:  fileStorage,
// 	}
// }

// // CreateDocument handles the complete document creation process
// func (s *DocumentService) CreateDocument(tx *gorm.DB, c *fiber.Ctx) (*CreateDocumentResponse, error) {
// 	config.Logger.Info("Document upload request received")

// 	// Parse multipart form
// 	form, err := c.MultipartForm()
// 	if err != nil {
// 		config.Logger.Error("Multipart form error", zap.Error(err))
// 		return nil, fmt.Errorf("multipart form error: %v", err)
// 	}

// 	// Process and validate file
// 	fileHeader, err := s.processFile(form)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Parse and validate metadata
// 	request, err := s.parseMetadata(form)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Validate the complete request
// 	if err := s.Validator.ValidateCreateDocumentRequest(request); err != nil {
// 		return nil, fmt.Errorf("validation error: %v", err)
// 	}

// 	// Process file upload
// 	filePath, downloadURL, err := s.saveFile(fileHeader, request)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Create document record
// 	document, err := s.createDocumentRecord(request, fileHeader, filePath, downloadURL)
// 	if err != nil {
// 		// Clean up uploaded file on database error
// 		if removeErr := os.Remove(filePath); removeErr != nil {
// 			config.Logger.Error("Failed to remove uploaded file after database error",
// 				zap.Error(removeErr), zap.String("file_path", filePath))
// 		}
// 		return nil, err
// 	}

// 	// Create the document in database
// 	createdDocument, err := s.DocumentRepo.CreateDocument(tx, document)
// 	if err != nil {
// 		// Clean up uploaded file on database error
// 		if removeErr := os.Remove(filePath); removeErr != nil {
// 			config.Logger.Error("Failed to remove uploaded file after database error",
// 				zap.Error(removeErr), zap.String("file_path", filePath))
// 		}
// 		return nil, fmt.Errorf("failed to create document: %v", err)
// 	}

// 	return &CreateDocumentResponse{
// 		Document:    createdDocument,
// 		DownloadURL: downloadURL,
// 	}, nil
// }

// // processFile validates and extracts file from multipart form
// func (s *DocumentService) processFile(form *multipart.Form) (*multipart.FileHeader, error) {
// 	files := form.File["files"]
// 	if len(files) != 1 {
// 		return nil, fmt.Errorf("exactly one file must be uploaded per request")
// 	}

// 	return files[0], nil
// }

// // parseMetadata parses and validates metadata from form
// func (s *DocumentService) parseMetadata(form *multipart.Form) (*documents_requests.CreateDocumentRequest, error) {
// 	metadataSlice := form.Value["metadata"]
// 	if len(metadataSlice) != 1 {
// 		return nil, fmt.Errorf("exactly one metadata payload must be provided")
// 	}

// 	request := new(documents_requests.CreateDocumentRequest)
// 	if err := json.Unmarshal([]byte(metadataSlice[0]), request); err != nil {
// 		return nil, fmt.Errorf("invalid metadata JSON: %v", err)
// 	}

// 	return request, nil
// }

// // createDocumentRecord creates a document model from the request
// func (s *DocumentService) createDocumentRecord(request *documents_requests.CreateDocumentRequest, fileHeader *multipart.FileHeader, filePath, downloadURL string) (*models.Document, error) {
// 	// Determine document type from file extension
// 	fileExtension := strings.ToLower(filepath.Ext(fileHeader.Filename))
// 	documentType, err := s.Validator.GetDocumentType(fileExtension)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Parse file size
// 	fileSizeDecimal, err := decimal.NewFromString(request.FileSize)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid file size format: %v", err)
// 	}

// 	// Clean filename for storage
// 	cleanedName := s.Validator.SanitizeFileName(request.FileName)
// 	fileExt := strings.ToLower(filepath.Ext(cleanedName))
// 	baseName := strings.TrimSuffix(cleanedName, fileExt)

// 	document := &models.Document{
// 		ID:                   uuid.New(),
// 		FileName:             baseName + fileExt,
// 		DocumentType:         documentType,
// 		FileSize:             fileSizeDecimal,
// 		DocumentCategory:     request.DocumentCategory,
// 		ClientID:             request.ClientID,
// 		CreatedBy:            request.CreatedBy,
// 		StandID:              request.StandID,
// 		PlanID:               request.PlanID,
// 		DevelopmentExpenseID: request.DevelopmentExpenseID,
// 		VendorID:             request.VendorID,
// 		FilePath:             downloadURL,
// 		CreatedAt:            time.Now(),
// 		UpdatedAt:            time.Now(),
// 	}

// 	return document, nil
// }

// func (s *DocumentService) CreateDocumentFromBytes(
// 	tx *gorm.DB,
// 	fileContent []byte,
// 	fileName string,
// 	fileSize string,
// 	metadata *documents_requests.CreateDocumentRequest,
// ) (*CreateDocumentResponse, error) {
// 	config.Logger.Info("Document creation from bytes request received")

// 	// Get file extension
// 	fileExt := filepath.Ext(fileName)
// 	if fileExt == "" {
// 		return nil, fmt.Errorf("file has no extension")
// 	}

// 	// Get document type and MIME type
// 	docType, err := s.Validator.GetDocumentType(fileExt)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid file type: %w", err)
// 	}

// 	// Map document type to MIME type
// 	mimeType, err := s.documentTypeToMimeType(docType)
// 	if err != nil {
// 		return nil, fmt.Errorf("unsupported document type: %w", err)
// 	}

// 	// Set the file type in metadata for validation
// 	metadata.FileType = mimeType

// 	// Validate metadata (now includes proper file type)
// 	if err := s.Validator.ValidateCreateDocumentRequest(metadata); err != nil {
// 		return nil, fmt.Errorf("validation error: %v", err)
// 	}
// 	// Create a temporary file header for processing compatibility
// 	fileHeader := &multipart.FileHeader{
// 		Filename: fileName,
// 		Size:     int64(len(fileContent)),
// 	}

// 	// Use existing saveFile method but we need to modify it to handle byte content
// 	originalName := strings.TrimSpace(metadata.FileName)
// 	cleanedName := s.Validator.SanitizeFileName(originalName)
// 	fileExt = strings.ToLower(filepath.Ext(cleanedName))
// 	baseName := strings.TrimSuffix(cleanedName, fileExt)

// 	timestamp := time.Now().Format("02_01_2006_03_04_05_PM")
// 	uniqueName := fmt.Sprintf("%s_%s%s", baseName, timestamp, fileExt)

// 	// Create a multipart.File compatible wrapper for the byte content
// 	src := newBytesFile(fileContent)

// 	// Use FileStorage to save the file (following the same pattern as CreateDocument)
// 	filePath, err := s.FileStorage.UploadFile(src, uniqueName)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to save file: %w", err)
// 	}

// 	downloadURL := fmt.Sprintf("/uploads/%s", uniqueName)

// 	// Create document record using existing method
// 	document, err := s.createDocumentRecord(metadata, fileHeader, filePath, downloadURL)
// 	if err != nil {
// 		// Clean up uploaded file on error
// 		if removeErr := os.Remove(filePath); removeErr != nil {
// 			config.Logger.Error("Failed to remove uploaded file after error",
// 				zap.Error(removeErr), zap.String("file_path", filePath))
// 		}
// 		return nil, err
// 	}

// 	// Create the document in database using existing repository method
// 	createdDocument, err := s.DocumentRepo.CreateDocument(tx, document)
// 	if err != nil {
// 		// Clean up uploaded file on database error
// 		if removeErr := os.Remove(filePath); removeErr != nil {
// 			config.Logger.Error("Failed to remove uploaded file after database error",
// 				zap.Error(removeErr), zap.String("file_path", filePath))
// 		}
// 		return nil, fmt.Errorf("failed to create document: %v", err)
// 	}

// 	return &CreateDocumentResponse{
// 		Document:    createdDocument,
// 		DownloadURL: downloadURL,
// 	}, nil
// }

// type bytesFile struct {
// 	*bytes.Reader
// 	content []byte
// }

// func (bf *bytesFile) Close() error {
// 	return nil
// }

// func (bf *bytesFile) ReadAt(p []byte, off int64) (n int, err error) {
// 	if off < 0 || off >= int64(len(bf.content)) {
// 		return 0, io.EOF
// 	}
// 	n = copy(p, bf.content[off:])
// 	if n < len(p) {
// 		err = io.EOF
// 	}
// 	return n, err
// }

// func newBytesFile(content []byte) *bytesFile {
// 	return &bytesFile{
// 		Reader:  bytes.NewReader(content),
// 		content: content,
// 	}
// }

// func (s *DocumentService) documentTypeToMimeType(docType models.DocumentType) (string, error) {
// 	mimeTypes := map[models.DocumentType]string{
// 		models.WordDocumentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
// 		models.TextDocumentType: "text/plain",
// 		models.SpreadsheetType:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
// 		models.PresentationType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
// 		models.ImageType:        "image/jpeg", // Default image type
// 		models.PDFType:          "application/pdf",
// 	}

// 	if mimeType, ok := mimeTypes[docType]; ok {
// 		return mimeType, nil
// 	}
// 	return "", fmt.Errorf("no MIME type mapping for document type: %s", docType)
// }

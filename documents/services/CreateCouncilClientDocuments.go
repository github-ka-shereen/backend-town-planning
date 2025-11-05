package services

// import (
// 	"town-planning-backend/config"
// 	"town-planning-backend/db/models"
// 	documents_requests "town-planning-backend/documents/requests"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"mime/multipart"
// 	"os"
// 	"path/filepath"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/google/uuid"
// 	"github.com/shopspring/decimal"
// 	"go.uber.org/zap"
// 	"gorm.io/gorm"
// )

// // CreateCouncilClientDocuments processes multiple files and their metadata within a transaction
// func (s *DocumentService) CreateCouncilClientDocuments(
// 	tx *gorm.DB,
// 	c *fiber.Ctx,
// 	clientID uuid.UUID,
// 	files []*multipart.FileHeader,
// 	metadataList []*documents_requests.CreateDocumentRequest,
// ) ([]*models.Document, error) {

// 	// Basic validation
// 	if len(files) != len(metadataList) {
// 		return nil, fmt.Errorf("mismatch between number of files (%d) and metadata entries (%d)",
// 			len(files), len(metadataList))
// 	}

// 	var createdDocuments []*models.Document

// 	for i, fileHeader := range files {
// 		meta := metadataList[i]
// 		meta.ClientID = &clientID

// 		// Create document with category lookup and audit trail
// 		docResponse, err := s.createDocumentWithCategoryLookup(tx, c, fileHeader, meta)
// 		if err != nil {
// 			config.Logger.Error("Failed to create document",
// 				zap.Error(err),
// 				zap.String("filename", fileHeader.Filename))
// 			return nil, fmt.Errorf("failed to process document %d (%s): %w",
// 				i+1, fileHeader.Filename, err)
// 		}

// 		// Create client-document relationship
// 		clientDoc := &models.ClientDocument{
// 			ID:            uuid.New(),
// 			ClientID:      clientID,
// 			DocumentID:    docResponse.ID,
// 			ApplicationID: meta.ApplicationID,
// 			CreatedBy:     meta.CreatedBy,
// 		}

// 		// Validate application if provided
// 		if meta.ApplicationID != nil {
// 			if err := s.validateApplication(tx, *meta.ApplicationID, clientID); err != nil {
// 				return nil, err
// 			}
// 		}

// 		config.Logger.Info("Creating client document",
// 			zap.String("client_id", clientID.String()),
// 			zap.Any("application_id", clientDoc.ApplicationID),
// 			zap.String("document_id", docResponse.ID.String()))

// 		if err := tx.Create(clientDoc).Error; err != nil {
// 			config.Logger.Error("Failed to create client document",
// 				zap.Error(err),
// 				zap.String("client_id", clientID.String()),
// 				zap.Any("application_id", clientDoc.ApplicationID))
// 			return nil, fmt.Errorf("failed to create client document relationship: %w", err)
// 		}

// 		config.Logger.Info("Client document created successfully",
// 			zap.String("client_document_id", clientDoc.ID.String()))

// 		if docResponse != nil && docResponse.Document != nil {
// 			createdDocuments = append(createdDocuments, docResponse.Document)
// 		} else {
// 			config.Logger.Warn("Unexpected nil document in response",
// 				zap.String("filename", fileHeader.Filename))
// 			return nil, fmt.Errorf("received nil document response for file: %s",
// 				fileHeader.Filename)
// 		}
// 	}

// 	return createdDocuments, nil
// }

// // createDocumentWithCategoryLookup handles document creation with category lookup and audit trail
// func (s *DocumentService) createDocumentWithCategoryLookup(
// 	tx *gorm.DB,
// 	c *fiber.Ctx,
// 	fileHeader *multipart.FileHeader,
// 	request *documents_requests.CreateDocumentRequest,
// ) (*CreateDocumentResponse, error) {

// 	// Validate request
// 	if err := s.Validator.ValidateCreateDocumentRequest(request); err != nil {
// 		return nil, fmt.Errorf("validation error for document (%s): %w",
// 			fileHeader.Filename, err)
// 	}

// 	// Look up category by code
// 	category, err := s.DocumentRepo.GetCategoryByCode(tx, request.CategoryCode)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to find category with code '%s': %w",
// 			request.CategoryCode, err)
// 	}

// 	// Save file
// 	filePath, downloadURL, err := s.saveFile(fileHeader, request)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to save file (%s): %w",
// 			fileHeader.Filename, err)
// 	}

// 	// Calculate file hash (simplified - you might want to use a proper hash)
// 	fileHash := s.calculateFileHash(fileHeader)

// 	// Create document record
// 	document := &models.Document{
// 		ID:            uuid.New(),
// 		FileName:      fileHeader.Filename,
// 		DocumentType:  models.DocumentType(request.FileType),
// 		FileSize:      decimal.NewFromInt(fileHeader.Size),
// 		CategoryID:    &category.ID, // Set the category ID from lookup
// 		ClientID:      request.ClientID,
// 		ApplicationID: request.ApplicationID,
// 		CreatedBy:     request.CreatedBy,
// 		FilePath:      filePath,
// 		FileHash:      fileHash,
// 		MimeType:      fileHeader.Header.Get("Content-Type"),
// 		Description:   &request.FileName, // Use the provided file name as description
// 		IsPublic:      false,             // Default to private
// 		IsMandatory:   true,              // Default to mandatory
// 		IsActive:      true,
// 	}

// 	// Extract user info for audit trail
// 	userID := request.CreatedBy
// 	userName := request.CreatedBy // You might want to get this from user context
// 	userRole := "user"            // You might want to get this from user context
// 	ipAddress := c.IP()
// 	userAgent := c.Get("User-Agent")

// 	// Create document with audit trail
// 	createdDocument, err := s.DocumentRepo.CreateDocumentWithAudit(
// 		tx,
// 		document,
// 		userID,
// 		userName,
// 		userRole,
// 		ipAddress,
// 		userAgent,
// 	)
// 	if err != nil {
// 		// Cleanup file if DB operation fails
// 		if removeErr := os.Remove(filePath); removeErr != nil {
// 			config.Logger.Error("Failed to remove uploaded file after DB error",
// 				zap.Error(removeErr),
// 				zap.String("file_path", filePath))
// 		}
// 		return nil, fmt.Errorf("failed to insert document into database (%s): %w",
// 			fileHeader.Filename, err)
// 	}

// 	config.Logger.Info("Document created successfully with audit trail",
// 		zap.String("document_id", createdDocument.ID.String()),
// 		zap.String("filename", fileHeader.Filename),
// 		zap.String("category_code", request.CategoryCode))

// 	return &CreateDocumentResponse{
// 		ID:          createdDocument.ID,
// 		Document:    createdDocument,
// 		DownloadURL: downloadURL,
// 	}, nil
// }

// // validateApplication validates that an application exists and belongs to the client
// func (s *DocumentService) validateApplication(tx *gorm.DB, applicationID uuid.UUID, clientID uuid.UUID) error {
// 	config.Logger.Info("Validating application existence",
// 		zap.String("application_id", applicationID.String()),
// 		zap.String("client_id", clientID.String()))

// 	var application models.Application
// 	result := tx.Unscoped().First(&application, "id = ?", applicationID)

// 	if result.Error != nil {
// 		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 			config.Logger.Error("Application not found",
// 				zap.String("application_id", applicationID.String()),
// 				zap.String("client_id", clientID.String()))
// 			return fmt.Errorf("application with ID %s does not exist", applicationID.String())
// 		}
// 		config.Logger.Error("Failed to query application",
// 			zap.Error(result.Error),
// 			zap.String("application_id", applicationID.String()))
// 		return fmt.Errorf("failed to verify application: %w", result.Error)
// 	}

// 	// Verify the application belongs to the correct client
// 	if application.ClientID != clientID {
// 		config.Logger.Error("Application client mismatch",
// 			zap.String("application_id", applicationID.String()),
// 			zap.String("expected_client_id", clientID.String()),
// 			zap.String("actual_client_id", application.ClientID.String()))
// 		return fmt.Errorf("application %s does not belong to client %s",
// 			applicationID.String(), clientID.String())
// 	}

// 	config.Logger.Info("Application validation successful",
// 		zap.String("application_id", applicationID.String()),
// 		zap.String("client_id", clientID.String()))
// 	return nil
// }

// // saveFile saves the uploaded file using the FileStorage interface
// func (s *DocumentService) saveFile(fileHeader *multipart.FileHeader, request *documents_requests.CreateDocumentRequest) (string, string, error) {
// 	originalName := strings.TrimSpace(request.FileName)
// 	cleanedName := s.Validator.SanitizeFileName(originalName)
// 	fileExt := strings.ToLower(filepath.Ext(cleanedName))
// 	baseName := strings.TrimSuffix(cleanedName, fileExt)

// 	timestamp := time.Now().Format("02_01_2006_03_04_05_PM")
// 	uniqueName := fmt.Sprintf("%s_%s%s", baseName, timestamp, fileExt)

// 	// Open the uploaded file
// 	src, err := fileHeader.Open()
// 	if err != nil {
// 		return "", "", fmt.Errorf("failed to open uploaded file: %w", err)
// 	}
// 	defer src.Close()

// 	// Use FileStorage to save the file
// 	filePath, err := s.FileStorage.UploadFile(src, uniqueName)
// 	if err != nil {
// 		return "", "", fmt.Errorf("failed to save file: %w", err)
// 	}

// 	downloadURL := fmt.Sprintf("/uploads/%s", uniqueName)
// 	return filePath, downloadURL, nil
// }

// // calculateFileHash generates a simple file hash (you might want to use a proper hash like SHA256)
// func (s *DocumentService) calculateFileHash(fileHeader *multipart.FileHeader) string {
// 	// This is a simplified hash - consider using proper file content hashing
// 	return fmt.Sprintf("%s-%d", fileHeader.Filename, fileHeader.Size)
// }

// // saveUploadedFile saves the uploaded file to the specified path
// func saveUploadedFile(fileHeader *multipart.FileHeader, dst string) error {
// 	src, err := fileHeader.Open()
// 	if err != nil {
// 		return err
// 	}
// 	defer src.Close()

// 	out, err := os.Create(dst)
// 	if err != nil {
// 		return err
// 	}
// 	defer out.Close()

// 	// Copy the file
// 	if _, err = io.Copy(out, src); err != nil {
// 		return err
// 	}

// 	return nil
// }

// // Helper function to read file content for hashing
// var ioCopy = func(dst io.Writer, src io.Reader) (int64, error) {
// 	return io.Copy(dst, src)
// }

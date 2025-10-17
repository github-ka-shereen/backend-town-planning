package controllers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UpdateApplicationRequest defines the request structure for updating an application
type UpdateApplicationRequest struct {
	ReceiptNumber string    `json:"receipt_number"`
	ReceiptDate   time.Time `json:"receipt_date"`
	UpdatedBy     string    `json:"updated_by"`
}

// DocumentUpload represents a file that needs to be processed
type DocumentUpload struct {
	FileHeader  *multipart.FileHeader
	DocType     models.DocumentType
	Description string
	IsMandatory bool
	FlagField   string
	FileContent []byte
	FileName    string
	PublicPath  string
	FullPath    string
}

// UpdateApplicationController handles updating application details and document uploads
func (ac *ApplicationController) UpdateApplicationController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Validate application ID
	if applicationID == "" {
		config.Logger.Error("Application ID is required")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Application ID is required",
			"error":   "missing_application_id",
		})
	}

	// Parse UUID
	appUUID, err := uuid.Parse(applicationID)
	if err != nil {
		config.Logger.Error("Invalid application ID format",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid application ID format",
			"error":   "invalid_uuid",
		})
	}

	// Parse form data FIRST - before transaction
	form, err := c.MultipartForm()
	if err != nil {
		config.Logger.Error("Failed to parse multipart form", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid form data",
			"error":   "invalid_form_data",
		})
	}

	// Extract basic fields
	var req UpdateApplicationRequest
	if receiptNumbers, exists := form.Value["receipt_number"]; exists && len(receiptNumbers) > 0 {
		req.ReceiptNumber = receiptNumbers[0]
	}
	if receiptDates, exists := form.Value["receipt_date"]; exists && len(receiptDates) > 0 {
		if parsedDate, err := time.Parse(time.RFC3339, receiptDates[0]); err == nil {
			req.ReceiptDate = parsedDate
		}
	}
	if updatedBys, exists := form.Value["updated_by"]; exists && len(updatedBys) > 0 {
		req.UpdatedBy = updatedBys[0]
	}

	// Start transaction - AFTER parsing form data
	tx := ac.DB.Session(&gorm.Session{}).WithContext(c.Context()).Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to start transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	txCommitted := false
	var pendingFileUploads []DocumentUpload // Track files that need to be saved after commit

	defer func() {
		if !txCommitted {
			// Transaction failed - rollback database changes
			if tx != nil {
				tx.Rollback()
				config.Logger.Warn("Transaction rolled back due to error")
			}
			// DO NOT save any files to disk since transaction failed
			config.Logger.Info("Cleaning up - no files saved to disk due to transaction failure")
		} else {
			// Transaction succeeded - now save files to disk
			ac.savePendingFilesToDisk(pendingFileUploads)
		}
	}()

	// Get existing application with relationships
	var existingApplication models.Application
	if err := tx.
		Preload("Documents", "is_active = ?", true).
		Preload("Payment").
		Preload("Tariff").
		First(&existingApplication, "id = ?", appUUID).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			config.Logger.Error("Application not found", zap.String("applicationID", applicationID))
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Application not found",
				"error":   "application_not_found",
			})
		}

		config.Logger.Error("Failed to fetch application",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch application details",
			"error":   err.Error(),
		})
	}

	// Prepare application update - only update the updated_by field
	applicationUpdate := map[string]interface{}{
		"updated_by": req.UpdatedBy,
	}

	// CREATE PAYMENT RECORD if receipt information is provided
	if req.ReceiptNumber != "" && !req.ReceiptDate.IsZero() {
		_, err := ac.createPaymentRecord(tx, &existingApplication, req, req.UpdatedBy)
		if err != nil {
			config.Logger.Error("Failed to create payment record",
				zap.String("applicationID", applicationID),
				zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create payment record",
				"error":   err.Error(),
			})
		}
	}

	// Document type mappings
	documentConfigs := []struct {
		formField    string
		documentType models.DocumentType
		flagField    string
		description  string
		isMandatory  bool
	}{
		{
			formField:    "scanned_receipt",
			documentType: models.ProcessedTPD1Form,
			flagField:    "ScannedReceiptProvided",
			description:  "Scanned receipt document",
			isMandatory:  true,
		},
		{
			formField:    "processed_tpd1_form",
			documentType: models.ProcessedTPD1Form,
			flagField:    "ScannedTPD1FormProvided",
			description:  "Processed TPD-1 form",
			isMandatory:  true,
		},
		{
			formField:    "processed_quotation",
			documentType: models.ProcessedDevelopmentPermitQuotation,
			flagField:    "QuotationProvided",
			description:  "Processed development permit quotation",
			isMandatory:  true,
		},
		{
			formField:    "scanned_initial_plan",
			documentType: models.InitialPlanDocument,
			flagField:    "ScannedInitialPlanProvided",
			description:  "Scanned initial plan",
			isMandatory:  true,
		},
		{
			formField:    "structural_engineering_certificate",
			documentType: models.StructuralEngineeringCertificateDocument,
			flagField:    "StructuralEngineeringCertificateProvided",
			description:  "Structural engineering certificate",
			isMandatory:  false,
		},
		{
			formField:    "ring_beam_certificate",
			documentType: models.RingBeamCertificateDocument,
			flagField:    "RingBeamCertificateProvided",
			description:  "Ring beam certificate",
			isMandatory:  false,
		},
	}

	// Process file uploads - READ FILES INTO MEMORY but don't save to disk yet
	for _, config := range documentConfigs {
		if files, exists := form.File[config.formField]; exists && len(files) > 0 {
			file := files[0]

			// Process file upload - read into memory and prepare document record
			documentUpload, _, err := ac.processDocumentUploadInMemory(tx, file, config.documentType, config.description, config.isMandatory, &existingApplication, req.UpdatedBy)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": fmt.Sprintf("Failed to process %s", config.formField),
					"error":   err.Error(),
				})
			}

			// Add to pending uploads (will be saved to disk after commit)
			pendingFileUploads = append(pendingFileUploads, *documentUpload)

			// Use the direct mapping function to get the correct database column name
			databaseColumn := getDatabaseColumnName(config.flagField)
			applicationUpdate[databaseColumn] = true

			// config.Logger.Info("Document processed in memory, ready for commit",
			// 	zap.String("fileName", documentUpload.FileName),
			// 	zap.String("databaseColumn", databaseColumn))
		}
	}

	// Update application record with document flags
	if len(applicationUpdate) > 0 {
		if err := tx.Model(&existingApplication).Updates(applicationUpdate).Error; err != nil {
			config.Logger.Error("Failed to update application",
				zap.String("applicationID", applicationID),
				zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to update application",
				"error":   err.Error(),
			})
		}
	}

	// Commit transaction - if this succeeds, then we'll save files to disk
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize application update",
			"error":   err.Error(),
		})
	}
	txCommitted = true

	config.Logger.Info("Application updated successfully",
		zap.String("applicationID", applicationID),
		zap.Any("updatedFields", applicationUpdate),
		zap.Int("pendingFileUploads", len(pendingFileUploads)))

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Application updated successfully",
		"data": fiber.Map{
			"application_id":     existingApplication.ID,
			"plan_number":        existingApplication.PlanNumber,
			"updated_fields":     applicationUpdate,
			"documents_uploaded": len(pendingFileUploads),
			"updated_at":         time.Now().Format(time.RFC3339),
		},
	})
}

// processDocumentUploadInMemory processes file upload in memory and creates document record WITHOUT saving to disk
func (ac *ApplicationController) processDocumentUploadInMemory(
	tx *gorm.DB,
	fileHeader *multipart.FileHeader,
	docType models.DocumentType,
	description string,
	isMandatory bool,
	application *models.Application,
	createdBy string,
) (*DocumentUpload, *models.Document, error) {

	// Generate unique filename
	fileExt := filepath.Ext(fileHeader.Filename)
	cleanName := strings.TrimSuffix(fileHeader.Filename, fileExt)
	cleanName = utils.CleanStringForFilename(cleanName)
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s%s", cleanName, timestamp, fileExt)

	// Define file paths
	uploadDir := filepath.Join("public", "uploads", "development_permit_application_documents", application.ID.String())
	fullPath := filepath.Join(uploadDir, filename)
	publicPath := strings.TrimPrefix(fullPath, "public/")

	// Read file content into memory
	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file content: %w", err)
	}

	// FIX: Generate file hash from memory content, NOT from disk
	fileHash, err := generateFileHashFromBytes(fileContent)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate file hash: %w", err)
	}

	// Create document record (in database only - file not saved to disk yet)
	document := models.Document{
		ID:               uuid.New(),
		ApplicationID:    &application.ID,
		FileName:         filename,
		DocumentType:     docType,
		FileSize:         int64(len(fileContent)),
		FilePath:         publicPath, // Store path without "public/" for frontend
		FileHash:         fileHash,
		MimeType:         fileHeader.Header.Get("Content-Type"),
		IsPublic:         true, // Make documents publicly accessible
		Description:      &description,
		IsMandatory:      isMandatory,
		IsActive:         true,
		CreatedBy:        createdBy,
		Version:          1,
		IsCurrentVersion: true,
		LastAction:       models.ActionCreate,
	}

	// Set original ID to point to itself for the first version
	document.OriginalID = &document.ID

	if err := tx.Create(&document).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create document record: %w", err)
	}

	// Create audit log
	auditLog := models.DocumentAuditLog{
		ID:         uuid.New(),
		DocumentID: document.ID,
		Action:     models.ActionCreate,
		UserID:     createdBy,
		Reason:     &description,
		Details:    &description,
		CreatedAt:  time.Now(),
	}

	if err := tx.Create(&auditLog).Error; err != nil {
		config.Logger.Warn("Failed to create document audit log",
			zap.String("documentID", document.ID.String()),
			zap.Error(err))
	}

	// Prepare document upload for later disk saving
	documentUpload := &DocumentUpload{
		FileHeader:  fileHeader,
		DocType:     docType,
		Description: description,
		IsMandatory: isMandatory,
		FileContent: fileContent,
		FileName:    filename,
		PublicPath:  publicPath,
		FullPath:    fullPath,
	}

	return documentUpload, &document, nil
}

// Add this helper function to your controller file
func generateFileHashFromBytes(fileContent []byte) (string, error) {
	hash := md5.New()
	hash.Write(fileContent)
	hashInBytes := hash.Sum(nil)[:16]
	return hex.EncodeToString(hashInBytes), nil
}

// savePendingFilesToDisk saves all pending file uploads to disk after successful transaction commit
func (ac *ApplicationController) savePendingFilesToDisk(uploads []DocumentUpload) {
	for _, upload := range uploads {
		// Create directory if it doesn't exist
		dir := filepath.Dir(upload.FullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			config.Logger.Error("Failed to create directory for file",
				zap.String("dir", dir),
				zap.Error(err))
			continue
		}

		// Save file to disk
		if err := os.WriteFile(upload.FullPath, upload.FileContent, 0644); err != nil {
			config.Logger.Error("Failed to save file to disk",
				zap.String("filePath", upload.FullPath),
				zap.Error(err))
			continue
		}

		config.Logger.Info("File saved to disk after successful transaction",
			zap.String("filePath", upload.FullPath),
			zap.Int("fileSize", len(upload.FileContent)))
	}
}

// createPaymentRecord creates a payment record for the application fee
func (ac *ApplicationController) createPaymentRecord(
	tx *gorm.DB,
	application *models.Application,
	req UpdateApplicationRequest,
	createdBy string,
) (*models.Payment, error) {

	// Calculate the amount based on application total cost
	amount := decimal.NewFromFloat(0)
	if application.TotalCost != nil {
		amount = *application.TotalCost
	}

	// Create payment record
	payment := models.Payment{
		ApplicationID:   &application.ID,
		TariffID:        application.TariffID,
		PaymentFor:      models.PaymentForApplicationFee,
		ReceiptNumber:   req.ReceiptNumber,
		PaymentDate:     req.ReceiptDate,
		Amount:          amount,
		PaymentMethod:   models.CashPaymentMethod, // Default to cash, can be enhanced
		PaymentStatus:   models.PaidPayment,
		TransactionType: models.OrdinaryTransactionType,
		CreatedBy:       createdBy,
	}

	// Use the BeforeCreate hook to generate TransactionNumber
	if err := payment.BeforeCreate(tx); err != nil {
		return nil, fmt.Errorf("failed to prepare payment: %w", err)
	}

	// Create the payment record
	if err := tx.Create(&payment).Error; err != nil {
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}

	config.Logger.Info("Payment record created",
		zap.String("paymentID", payment.ID.String()),
		zap.String("receiptNumber", payment.ReceiptNumber),
		zap.String("amount", payment.Amount.String()))

	return &payment, nil
}

// Helper function to convert CamelCase to snake_case
func toSnakeCase(str string) string {
	var result strings.Builder
	for i, char := range str {
		if i > 0 && 'A' <= char && char <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(char)
	}
	return strings.ToLower(result.String())
}

// Helper function to map flag fields to actual database column names
func getDatabaseColumnName(flagField string) string {
	// Direct mapping to avoid any conversion issues
	fieldMap := map[string]string{
		"ScannedReceiptProvided":                   "scanned_receipt_provided",
		"ScannedTPD1FormProvided":                  "scanned_tpd1_form_provided",
		"QuotationProvided":                        "quotation_provided",
		"ScannedInitialPlanProvided":               "scanned_initial_plan_provided",
		"StructuralEngineeringCertificateProvided": "structural_engineering_certificate_provided",
		"RingBeamCertificateProvided":              "ring_beam_certificate_provided",
	}

	if column, exists := fieldMap[flagField]; exists {
		return column
	}

	// Fallback to simple conversion if somehow we get an unknown field
	return strings.ToLower(flagField)
}

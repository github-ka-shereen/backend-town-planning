package controllers

import (
	"fmt"
	"mime/multipart"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	documents_requests "town-planning-backend/documents/requests"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ApplicationUploadRequest handles both files and flags
type ApplicationUploadRequest struct {
	ReceiptNumber string `form:"receipt_number"`
	ReceiptDate   string `form:"receipt_date"`
	UpdatedBy     string `form:"updated_by"`

	// Files
	ScannedReceipt                   *multipart.FileHeader `form:"scanned_receipt"`
	ProcessedTPD1Form                *multipart.FileHeader `form:"processed_tpd1_form"`
	ProcessedQuotation               *multipart.FileHeader `form:"processed_quotation"`
	ScannedInitialPlan               *multipart.FileHeader `form:"scanned_initial_plan"`
	StructuralEngineeringCertificate *multipart.FileHeader `form:"structural_engineering_certificate"`
	RingBeamCertificate              *multipart.FileHeader `form:"ring_beam_certificate"`

	// Flags (as strings from form data)
	ProcessedReceiptProvided                 string `form:"processed_receipt_provided"`
	InitialPlanProvided                      string `form:"initial_plan_provided"`
	ProcessedTPD1FormProvided                string `form:"processed_tpd1_form_provided"`
	ProcessedQuotationProvided               string `form:"processed_quotation_provided"`
	StructuralEngineeringCertificateProvided string `form:"structural_engineering_certificate_provided"`
	RingBeamCertificateProvided              string `form:"ring_beam_certificate_provided"`
}

// ProcessApplicationSubmissionController handles both file uploads, flag updates, and payment creation
func (ac *ApplicationController) ProcessApplicationSubmissionController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Get authenticated user
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	// Parse application ID
	appUUID, err := uuid.Parse(applicationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid application ID",
			"error":   "invalid_uuid",
		})
	}

	// Parse multipart form
	var req ApplicationUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Validate required fields if receipt is being processed
	if req.ReceiptNumber != "" && req.ReceiptDate == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Receipt date is required when receipt number is provided",
		})
	}

	// Start transaction
	tx := ac.DB.Session(&gorm.Session{}).WithContext(c.Context()).Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to start transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Transaction panic recovery", zap.Any("panic", r))
		}
	}()

	// Get existing application with relationships
	var application models.Application
	if err := tx.
		Preload("Tariff").
		Preload("Payment").
		First(&application, "id = ?", appUUID).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Application not found",
			"error":   "application_not_found",
		})
	}

	// Process file uploads and update flags
	updates := make(map[string]interface{})
	updatedByStr := payload.UserID.String()
	updates["updated_by"] = updatedByStr

	var uploadedDocuments []string
	var receiptProvided bool

	// Map files to document categories and flags
	fileMappings := []struct {
		fileHeader *multipart.FileHeader
		category   string
		flagField  string
		flagValue  string
	}{
		{req.ScannedReceipt, "PROCESSED_RECEIPT", "processed_receipt_provided", req.ProcessedReceiptProvided},
		{req.ProcessedTPD1Form, "TPD1_FORM", "processed_tpd1_form_provided", req.ProcessedTPD1FormProvided},
		{req.ProcessedQuotation, "QUOTATION", "processed_quotation_provided", req.ProcessedQuotationProvided},
		{req.ScannedInitialPlan, "INITIAL_PLAN", "initial_plan_provided", req.InitialPlanProvided},
		{req.StructuralEngineeringCertificate, "ENGINEERING_CERTIFICATE", "structural_engineering_certificate_provided", req.StructuralEngineeringCertificateProvided},
		{req.RingBeamCertificate, "RING_BEAM_CERTIFICATE", "ring_beam_certificate_provided", req.RingBeamCertificateProvided},
	}

	// Process all files first - if any fail, rollback entire transaction
	for _, mapping := range fileMappings {
		// Determine flag value: true if file is provided OR flag is explicitly set to "true"
		flagValue := mapping.fileHeader != nil || mapping.flagValue == "true"
		
		if mapping.fileHeader != nil {
			// Upload the document
			docRequest := &documents_requests.CreateDocumentRequest{
				CategoryCode:  mapping.category,
				FileName:      mapping.fileHeader.Filename,
				ApplicationID: &appUUID,
				ApplicantID:   &application.ApplicantID,
				CreatedBy:     payload.UserID.String(),
				FileType:      mapping.fileHeader.Header.Get("Content-Type"),
			}

			// Read file content
			file, err := mapping.fileHeader.Open()
			if err != nil {
				tx.Rollback()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": fmt.Sprintf("Failed to open file: %s", mapping.fileHeader.Filename),
					"error":   err.Error(),
				})
			}

			fileContent := make([]byte, mapping.fileHeader.Size)
			if _, err := file.Read(fileContent); err != nil {
				file.Close()
				tx.Rollback()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": fmt.Sprintf("Failed to read file: %s", mapping.fileHeader.Filename),
					"error":   err.Error(),
				})
			}
			file.Close()

			// Create document using DocumentService - if this fails, rollback
			_, err = ac.DocumentSvc.UnifiedCreateDocument(tx, c, docRequest, fileContent, nil)
			if err != nil {
				tx.Rollback()
				config.Logger.Error("Failed to create document",
					zap.String("filename", mapping.fileHeader.Filename),
					zap.Error(err))
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": fmt.Sprintf("Failed to upload document: %s", mapping.fileHeader.Filename),
					"error":   err.Error(),
				})
			}

			uploadedDocuments = append(uploadedDocuments, mapping.fileHeader.Filename)

			// Track if receipt was provided
			if mapping.category == "PROCESSED_RECEIPT" {
				receiptProvided = true
			}
		}

		// Update the flag value (true if file was provided OR flag was explicitly set)
		updates[mapping.flagField] = flagValue
	}

	// Handle payment creation if receipt is provided
	if receiptProvided && req.ReceiptNumber != "" && req.ReceiptDate != "" {
		payment, err := ac.createPaymentRecordFromReceipt(tx, &application, req, updatedByStr)
		if err != nil {
			tx.Rollback()
			config.Logger.Error("Failed to create payment record",
				zap.String("applicationID", applicationID),
				zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create payment record",
				"error":   err.Error(),
			})
		}

		// Update payment status and payment completion date
		updates["payment_status"] = models.PaidPayment
		now := time.Now()
		updates["payment_completed_at"] = &now

		config.Logger.Info("Payment record created from receipt",
			zap.String("paymentID", payment.ID.String()),
			zap.String("receiptNumber", payment.ReceiptNumber),
			zap.String("amount", payment.Amount.String()))
	}

	// Update all documents provided flag
	ac.updateAllDocumentsProvidedFlag(&application, updates)

	// Update ready for review flag
	ac.updateReadyForReviewFlag(&application, updates)

	// Apply updates to application
	if len(updates) > 0 {
		if err := tx.Model(&application).Updates(updates).Error; err != nil {
			tx.Rollback()
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

	// Commit transaction - if this fails, everything rolls back automatically
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize document upload",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Application documents uploaded successfully",
		zap.String("applicationID", applicationID),
		zap.Int("documentsUploaded", len(uploadedDocuments)),
		zap.Bool("paymentCreated", receiptProvided && req.ReceiptNumber != ""))

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Documents uploaded and payment processed successfully",
		"data": fiber.Map{
			"application_id":     applicationID,
			"uploaded_documents": uploadedDocuments,
			"payment_created":    receiptProvided && req.ReceiptNumber != "",
			"updated_flags":      updates,
			"processed_at":       time.Now().Format(time.RFC3339),
		},
	})
}

// createPaymentRecordFromReceipt creates a payment record when receipt is provided
func (ac *ApplicationController) createPaymentRecordFromReceipt(
	tx *gorm.DB,
	application *models.Application,
	req ApplicationUploadRequest,
	createdBy string,
) (*models.Payment, error) {

	// Parse receipt date
	receiptDate, err := time.Parse(time.RFC3339, req.ReceiptDate)
	if err != nil {
		return nil, fmt.Errorf("invalid receipt date format: %w", err)
	}

	// Validate receipt date is not in the future
	if receiptDate.After(time.Now()) {
		return nil, fmt.Errorf("receipt date cannot be in the future")
	}

	// Calculate the amount based on application total cost
	amount := decimal.NewFromFloat(0)
	if application.TotalCost != nil {
		amount = *application.TotalCost
	} else {
		// Fallback to estimated cost if total cost is not set
		if application.EstimatedCost != nil {
			amount = *application.EstimatedCost
		}
	}

	// Check if payment already exists for this application with transaction lock
	var existingPayment models.Payment
	err = tx.Set("gorm:query_option", "FOR UPDATE").
		Where("application_id = ? AND payment_for = ?", application.ID, models.PaymentForApplicationFee).
		First(&existingPayment).Error

	if err == nil {
		// Payment exists, update it
		existingPayment.ReceiptNumber = req.ReceiptNumber
		existingPayment.PaymentDate = receiptDate
		existingPayment.Amount = amount
		existingPayment.PaymentStatus = models.PaidPayment
		existingPayment.UpdatedAt = time.Now()

		if err := tx.Save(&existingPayment).Error; err != nil {
			return nil, fmt.Errorf("failed to update payment record: %w", err)
		}

		config.Logger.Info("Payment record updated",
			zap.String("paymentID", existingPayment.ID.String()),
			zap.String("receiptNumber", existingPayment.ReceiptNumber))

		return &existingPayment, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing payment: %w", err)
	}

	// Create new payment record
	payment := models.Payment{
		ApplicationID:   &application.ID,
		TariffID:        application.TariffID,
		PaymentFor:      models.PaymentForApplicationFee,
		ReceiptNumber:   req.ReceiptNumber,
		PaymentDate:     receiptDate,
		Amount:          amount,
		PaymentMethod:   models.CashPaymentMethod, // Default to cash
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

	return &payment, nil
}

// updateAllDocumentsProvidedFlag checks if all mandatory documents are provided
func (ac *ApplicationController) updateAllDocumentsProvidedFlag(
	application *models.Application,
	updates map[string]interface{},
) {
	// Get current values or updated values
	getValue := func(field string, current bool) bool {
		if val, exists := updates[field]; exists {
			if boolVal, ok := val.(bool); ok {
				return boolVal
			}
		}
		return current
	}

	processedReceipt := getValue("processed_receipt_provided", application.ProcessedReceiptProvided)
	initialPlan := getValue("initial_plan_provided", application.InitialPlanProvided)
	tpd1Form := getValue("processed_tpd1_form_provided", application.ProcessedTPD1FormProvided)
	quotation := getValue("processed_quotation_provided", application.ProcessedQuotationProvided)

	allDocsProvided := processedReceipt && initialPlan && tpd1Form && quotation

	updates["all_documents_provided"] = allDocsProvided

	if allDocsProvided && application.DocumentsCompletedAt == nil {
		now := time.Now()
		updates["documents_completed_at"] = &now
	}
}

// updateReadyForReviewFlag checks if application is ready for review
func (ac *ApplicationController) updateReadyForReviewFlag(
	application *models.Application,
	updates map[string]interface{},
) {
	// Get payment status
	paymentStatus := application.PaymentStatus
	if val, exists := updates["payment_status"]; exists {
		if status, ok := val.(models.PaymentStatus); ok {
			paymentStatus = status
		}
	}

	// Get all documents provided status
	allDocsProvided := application.AllDocumentsProvided
	if val, exists := updates["all_documents_provided"]; exists {
		if provided, ok := val.(bool); ok {
			allDocsProvided = provided
		}
	}

	readyForReview := paymentStatus == models.PaidPayment && allDocsProvided

	updates["ready_for_review"] = readyForReview

	if readyForReview && application.ReviewStartedAt == nil {
		now := time.Now()
		updates["review_started_at"] = &now
	}
}
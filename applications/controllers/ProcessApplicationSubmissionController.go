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

	// Process file uploads and update flags - ONLY for provided fields
	updates := make(map[string]interface{})
	updatedByStr := payload.UserID.String()
	updates["updated_by"] = updatedByStr

	var uploadedDocuments []string
	var receiptProvided bool

	// Debug: Log what we received
	config.Logger.Info("Processing application submission",
		zap.String("applicationID", applicationID),
		zap.String("receiptNumber", req.ReceiptNumber),
		zap.String("receiptDate", req.ReceiptDate),
		zap.Bool("hasScannedReceipt", req.ScannedReceipt != nil),
		zap.String("processedReceiptFlag", req.ProcessedReceiptProvided))

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

	// Process only provided files and flags
	for _, mapping := range fileMappings {
		fileProvided := mapping.fileHeader != nil
		flagExplicitlySet := mapping.flagValue == "true"
		
		// Only process if file was provided OR flag was explicitly set
		if fileProvided || flagExplicitlySet {
			flagValue := fileProvided || flagExplicitlySet
			
			if fileProvided {
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

				// Create document using DocumentService
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
				config.Logger.Info("Document uploaded successfully",
					zap.String("filename", mapping.fileHeader.Filename),
					zap.String("category", mapping.category))

				// Track if receipt was provided
				if mapping.category == "PROCESSED_RECEIPT" {
					receiptProvided = true
				}
			}

			// Only update the flag if we have a file OR explicit flag
			updates[mapping.flagField] = flagValue
			config.Logger.Info("Flag updated",
				zap.String("field", mapping.flagField),
				zap.Bool("value", flagValue),
				zap.Bool("fileProvided", fileProvided),
				zap.Bool("flagExplicitlySet", flagExplicitlySet))
		} else {
			config.Logger.Info("Skipping field - no file or explicit flag",
				zap.String("field", mapping.flagField))
		}
	}

	// Handle payment creation if receipt details are provided (with or without file)
	paymentCreated := false
	if req.ReceiptNumber != "" && req.ReceiptDate != "" {
		config.Logger.Info("Processing payment creation",
			zap.String("receiptNumber", req.ReceiptNumber),
			zap.Bool("receiptFileProvided", receiptProvided))

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
		paymentCreated = true

		config.Logger.Info("Payment record created from receipt",
			zap.String("paymentID", payment.ID.String()),
			zap.String("receiptNumber", payment.ReceiptNumber),
			zap.String("amount", payment.Amount.String()))
	} else {
		config.Logger.Info("Skipping payment creation - missing receipt details")
	}

	// Only update calculated flags if we have relevant updates
	if len(updates) > 1 { // More than just updated_by
		ac.updateAllDocumentsProvidedFlag(&application, updates)
		ac.updateReadyForReviewFlag(&application, updates)
	}

	// Apply updates to application only if we have updates beyond updated_by
	if len(updates) > 1 {
		config.Logger.Info("Applying updates to application", zap.Any("updates", updates))
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
	} else {
		config.Logger.Info("No updates to apply beyond updated_by")
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize document upload",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Application documents processed successfully",
		zap.String("applicationID", applicationID),
		zap.Int("documentsUploaded", len(uploadedDocuments)),
		zap.Bool("paymentCreated", paymentCreated))

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Documents uploaded and payment processed successfully",
		"data": fiber.Map{
			"application_id":     applicationID,
			"uploaded_documents": uploadedDocuments,
			"payment_created":    paymentCreated,
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
		config.Logger.Info("Updating existing payment record",
			zap.String("paymentID", existingPayment.ID.String()),
			zap.String("oldReceiptNumber", existingPayment.ReceiptNumber),
			zap.String("newReceiptNumber", req.ReceiptNumber))

		existingPayment.ReceiptNumber = req.ReceiptNumber
		existingPayment.PaymentDate = receiptDate
		existingPayment.Amount = amount
		existingPayment.PaymentStatus = models.PaidPayment
		existingPayment.UpdatedAt = time.Now()

		if err := tx.Save(&existingPayment).Error; err != nil {
			return nil, fmt.Errorf("failed to update payment record: %w", err)
		}

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

	config.Logger.Info("New payment record created",
		zap.String("paymentID", payment.ID.String()),
		zap.String("receiptNumber", payment.ReceiptNumber))

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

	config.Logger.Info("Documents provided status",
		zap.Bool("processedReceipt", processedReceipt),
		zap.Bool("initialPlan", initialPlan),
		zap.Bool("tpd1Form", tpd1Form),
		zap.Bool("quotation", quotation),
		zap.Bool("allDocsProvided", allDocsProvided))
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

	config.Logger.Info("Review readiness status",
		zap.String("paymentStatus", string(paymentStatus)),
		zap.Bool("allDocsProvided", allDocsProvided),
		zap.Bool("readyForReview", readyForReview))
}
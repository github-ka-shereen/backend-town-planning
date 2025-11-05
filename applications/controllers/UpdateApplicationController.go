// controllers/application_update_controller.go
package controllers

import (
	"fmt"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UpdateApplicationRequest defines the structure for application updates (from existing controller)
type UpdateApplicationRequest struct {
	ReceiptNumber string    `json:"receipt_number" form:"receipt_number"`
	ReceiptDate   time.Time `json:"receipt_date" form:"receipt_date"`
	UpdatedBy     string    `json:"updated_by" form:"updated_by"`

	// Document flags
	ProcessedReceiptProvided                 *bool `json:"processed_receipt_provided" form:"processed_receipt_provided"`
	InitialPlanProvided                      *bool `json:"initial_plan_provided" form:"initial_plan_provided"`
	ProcessedTPD1FormProvided                *bool `json:"processed_tpd1_form_provided" form:"processed_tpd1_form_provided"`
	ProcessedQuotationProvided               *bool `json:"processed_quotation_provided" form:"processed_quotation_provided"`
	StructuralEngineeringCertificateProvided *bool `json:"structural_engineering_certificate_provided" form:"structural_engineering_certificate_provided"`
	RingBeamCertificateProvided              *bool `json:"ring_beam_certificate_provided" form:"ring_beam_certificate_provided"`
}

// UpdateApplicationDetailsRequest defines the comprehensive update structure
type UpdateApplicationDetailsRequest struct {
	// Architect Information
	ArchitectFullName    *string `json:"architect_full_name"`
	ArchitectEmail       *string `json:"architect_email"`
	ArchitectPhoneNumber *string `json:"architect_phone_number"`

	// Planning Details
	PlanArea *decimal.Decimal `json:"plan_area"`

	// Property References
	PropertyTypeID *uuid.UUID `json:"property_type_id"`
	StandID        *uuid.UUID `json:"stand_id"`

	// Tariff and Financial
	TariffID      *uuid.UUID       `json:"tariff_id"`
	VATRateID     *uuid.UUID       `json:"vat_rate_id"`
	EstimatedCost *decimal.Decimal `json:"estimated_cost"`

	// Payment Information
	ReceiptNumber *string    `json:"receipt_number"`
	ReceiptDate   *time.Time `json:"receipt_date"`

	// Status Updates
	Status            *models.ApplicationStatus `json:"status"`
	PaymentStatus     *models.PaymentStatus     `json:"payment_status"`
	IsCollected       *bool                     `json:"is_collected"`
	CollectedBy       *string                   `json:"collected_by"`
	CollectionDate    *time.Time                `json:"collection_date"`
	FinalApprovalDate *time.Time                `json:"final_approval_date"`

	// Approval Group Assignment
	AssignedGroupID *uuid.UUID `json:"assigned_group_id"`
	FinalApproverID *uuid.UUID `json:"final_approver_id"`

	// Document Flags (if updating document status separately)
	ProcessedReceiptProvided                 *bool `json:"processed_receipt_provided"`
	InitialPlanProvided                      *bool `json:"initial_plan_provided"`
	ProcessedTPD1FormProvided                *bool `json:"processed_tpd1_form_provided"`
	ProcessedQuotationProvided               *bool `json:"processed_quotation_provided"`
	StructuralEngineeringCertificateProvided *bool `json:"structural_engineering_certificate_provided"`
	RingBeamCertificateProvided              *bool `json:"ring_beam_certificate_provided"`
}

// UpdateApplicationDetailsController handles comprehensive application updates
func (ac *ApplicationController) UpdateApplicationDetailsController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Get authenticated user
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	userUUID := payload.UserID

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

	// Parse request body
	var req UpdateApplicationDetailsRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Failed to parse request body", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
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

	// Get existing application
	var existingApplication models.Application
	if err := tx.
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Preload("Documents", "is_active = ?", true).
		Preload("Payment").
		First(&existingApplication, "id = ?", appUUID).Error; err != nil {

		tx.Rollback()
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

	// Validate application can be updated (not collected or expired)
	if err := ac.validateApplicationForUpdate(&existingApplication); err != nil {
		tx.Rollback()
		config.Logger.Error("Application cannot be updated",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
			"error":   "application_not_updatable",
		})
	}

	// Build update map
	updates := make(map[string]interface{})
	updatedByStr := userUUID.String()
	updates["updated_by"] = updatedByStr

	// Update architect information
	if req.ArchitectFullName != nil {
		updates["architect_full_name"] = *req.ArchitectFullName
	}
	if req.ArchitectEmail != nil {
		updates["architect_email"] = *req.ArchitectEmail
	}
	if req.ArchitectPhoneNumber != nil {
		updates["architect_phone_number"] = *req.ArchitectPhoneNumber
	}

	// Update planning details
	if req.PlanArea != nil {
		updates["plan_area"] = req.PlanArea
	}

	// Update property references
	if req.PropertyTypeID != nil {
		updates["property_type_id"] = req.PropertyTypeID
	}
	if req.StandID != nil {
		updates["stand_id"] = req.StandID
	}

	// Update financial details
	if req.EstimatedCost != nil {
		updates["estimated_cost"] = req.EstimatedCost
	}

	// Handle tariff change and recalculate costs
	tariffChanged := false
	if req.TariffID != nil && (existingApplication.TariffID == nil || *req.TariffID != *existingApplication.TariffID) {
		updates["tariff_id"] = req.TariffID
		tariffChanged = true
	}

	// Handle VAT rate change
	vatRateChanged := false
	if req.VATRateID != nil && (existingApplication.VATRateID == nil || *req.VATRateID != *existingApplication.VATRateID) {
		updates["vat_rate_id"] = req.VATRateID
		vatRateChanged = true
	}

	// Recalculate costs if tariff or VAT changed or plan area updated
	if tariffChanged || vatRateChanged || req.PlanArea != nil {
		if err := ac.recalculateApplicationCosts(tx, &existingApplication, updates, req); err != nil {
			tx.Rollback()
			config.Logger.Error("Failed to recalculate costs",
				zap.String("applicationID", applicationID),
				zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to recalculate application costs",
				"error":   err.Error(),
			})
		}
	}

	// Update status fields
	if req.Status != nil {
		updates["status"] = req.Status
		ac.updateWorkflowTimestamps(updates, *req.Status, &existingApplication)
	}

	if req.PaymentStatus != nil {
		updates["payment_status"] = req.PaymentStatus
		if *req.PaymentStatus == models.PaidPayment {
			now := time.Now()
			updates["payment_completed_at"] = &now
		}
	}

	// Update collection information
	if req.IsCollected != nil {
		updates["is_collected"] = req.IsCollected
		if *req.IsCollected {
			if req.CollectedBy != nil {
				updates["collected_by"] = req.CollectedBy
			}
			if req.CollectionDate != nil {
				updates["collection_date"] = req.CollectionDate
			} else {
				now := time.Now()
				updates["collection_date"] = &now
			}
			// Auto-set status to COLLECTED if not already set
			if req.Status == nil || *req.Status != models.CollectedApplication {
				updates["status"] = models.CollectedApplication
				ac.updateWorkflowTimestamps(updates, models.CollectedApplication, &existingApplication)
			}
		}
	}

	// Update approval dates
	if req.FinalApprovalDate != nil {
		updates["final_approval_date"] = req.FinalApprovalDate
	}

	// Update approval group assignments
	if req.AssignedGroupID != nil {
		updates["assigned_group_id"] = req.AssignedGroupID
	}
	if req.FinalApproverID != nil {
		updates["final_approver_id"] = req.FinalApproverID
	}

	// Update document flags
	ac.updateDocumentFlags(&existingApplication, updates, req)

	// Check if all mandatory documents are provided
	ac.updateAllDocumentsProvidedFlag(&existingApplication, updates)

	// Check if ready for review (payment complete + all docs provided)
	ac.updateReadyForReviewFlag(&existingApplication, updates)

	// Create payment record if receipt information provided
	if req.ReceiptNumber != nil && req.ReceiptDate != nil {
		paymentReq := UpdateApplicationRequest{
			ReceiptNumber: *req.ReceiptNumber,
			ReceiptDate:   *req.ReceiptDate,
			UpdatedBy:     updatedByStr,
		}
		_, err := ac.createPaymentRecord(tx, &existingApplication, paymentReq, updatedByStr)
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
	}

	// Apply updates to application
	if len(updates) > 0 {
		if err := tx.Model(&existingApplication).Updates(updates).Error; err != nil {
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

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize application update",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Application updated successfully",
		zap.String("applicationID", applicationID),
		zap.String("updatedBy", updatedByStr),
		zap.Int("fieldsUpdated", len(updates)))

	// Reload application with updated data
	if err := ac.DB.
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		First(&existingApplication, "id = ?", appUUID).Error; err != nil {
		config.Logger.Warn("Failed to reload application after update", zap.Error(err))
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Application updated successfully",
		"data": fiber.Map{
			"application":    existingApplication,
			"updated_fields": updates,
			"updated_at":     time.Now().Format(time.RFC3339),
		},
	})
}

// validateApplicationForUpdate checks if application can be updated
func (ac *ApplicationController) validateApplicationForUpdate(application *models.Application) error {
	// Applications that are collected cannot be updated
	if application.IsCollected {
		return fmt.Errorf("application has been collected and cannot be modified")
	}

	// Applications that are expired cannot be updated
	if application.Status == models.ExpiredApplication {
		return fmt.Errorf("application has expired and cannot be modified")
	}

	return nil
}

// updateDocumentFlags updates document flag fields
func (ac *ApplicationController) updateDocumentFlags(
	application *models.Application,
	updates map[string]interface{},
	req UpdateApplicationDetailsRequest,
) {
	if req.ProcessedReceiptProvided != nil {
		updates["processed_receipt_provided"] = *req.ProcessedReceiptProvided
	}
	if req.InitialPlanProvided != nil {
		updates["initial_plan_provided"] = *req.InitialPlanProvided
	}
	if req.ProcessedTPD1FormProvided != nil {
		updates["processed_tpd1_form_provided"] = *req.ProcessedTPD1FormProvided
	}
	if req.ProcessedQuotationProvided != nil {
		updates["processed_quotation_provided"] = *req.ProcessedQuotationProvided
	}
	if req.StructuralEngineeringCertificateProvided != nil {
		updates["structural_engineering_certificate_provided"] = *req.StructuralEngineeringCertificateProvided
	}
	if req.RingBeamCertificateProvided != nil {
		updates["ring_beam_certificate_provided"] = *req.RingBeamCertificateProvided
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

// recalculateApplicationCosts recalculates all financial fields
func (ac *ApplicationController) recalculateApplicationCosts(
	tx *gorm.DB,
	application *models.Application,
	updates map[string]interface{},
	req UpdateApplicationDetailsRequest,
) error {
	// Get current or new tariff
	var tariff models.Tariff
	tariffID := req.TariffID
	if tariffID == nil {
		tariffID = application.TariffID
	}

	if tariffID == nil {
		return fmt.Errorf("no tariff specified for cost calculation")
	}

	if err := tx.Preload("DevelopmentCategory").First(&tariff, "id = ?", tariffID).Error; err != nil {
		return fmt.Errorf("failed to fetch tariff: %w", err)
	}

	// Get current or new VAT rate
	var vatRate models.VATRate
	vatRateID := req.VATRateID
	if vatRateID == nil {
		vatRateID = application.VATRateID
	}

	if vatRateID == nil {
		return fmt.Errorf("no VAT rate specified for cost calculation")
	}

	if err := tx.First(&vatRate, "id = ?", vatRateID).Error; err != nil {
		return fmt.Errorf("failed to fetch VAT rate: %w", err)
	}

	// Get plan area
	planArea := req.PlanArea
	if planArea == nil {
		planArea = application.PlanArea
	}

	if planArea == nil || planArea.IsZero() {
		return fmt.Errorf("plan area is required for cost calculation")
	}

	// Calculate costs
	// Area cost = PlanArea × PricePerSquareMeter
	areaCost := planArea.Mul(tariff.PricePerSquareMeter)

	// Development levy = (AreaCost + PermitFee + InspectionFee) × DevelopmentLevyPercent / 100
	subtotal := areaCost.Add(tariff.PermitFee).Add(tariff.InspectionFee)
	developmentLevy := subtotal.Mul(tariff.DevelopmentLevyPercent).Div(decimal.NewFromInt(100))

	// Total before VAT
	totalBeforeVAT := subtotal.Add(developmentLevy)

	// VAT amount
	vatAmount := totalBeforeVAT.Mul(vatRate.Rate)

	// Total cost
	totalCost := totalBeforeVAT.Add(vatAmount)

	// Update the updates map
	updates["development_levy"] = developmentLevy
	updates["vat_amount"] = vatAmount
	updates["total_cost"] = totalCost

	config.Logger.Info("Recalculated application costs",
		zap.String("planArea", planArea.String()),
		zap.String("developmentLevy", developmentLevy.String()),
		zap.String("vatAmount", vatAmount.String()),
		zap.String("totalCost", totalCost.String()))

	return nil
}

// updateWorkflowTimestamps sets appropriate timestamp fields based on status
func (ac *ApplicationController) updateWorkflowTimestamps(
	updates map[string]interface{},
	status models.ApplicationStatus,
	application *models.Application,
) {
	now := time.Now()

	switch status {
	case models.UnderReviewApplication:
		if application.ReviewStartedAt == nil {
			updates["review_started_at"] = &now
		}
	case models.ApprovedApplication:
		if application.FinalApprovalDate == nil {
			updates["final_approval_date"] = &now
		}
		if application.ReviewCompletedAt == nil {
			updates["review_completed_at"] = &now
		}
	case models.RejectedApplication:
		if application.RejectionDate == nil {
			updates["rejection_date"] = &now
		}
		if application.ReviewCompletedAt == nil {
			updates["review_completed_at"] = &now
		}
	case models.CollectedApplication:
		if application.CollectionDate == nil {
			updates["collection_date"] = &now
		}
	}
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
}

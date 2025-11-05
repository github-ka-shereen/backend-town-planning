// controllers/application_controller.go
package controllers

import (
	"fmt"
	"os"
	"strings"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	documents_requests "town-planning-backend/documents/requests"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateApplicationRequest represents the request payload for creating an application
type CreateApplicationRequest struct {
	PlanArea             decimal.Decimal `json:"plan_area" validate:"required,min=0"`
	ArchitectFullName    *string         `json:"architect_full_name"`
	ArchitectEmail       *string         `json:"architect_email"`
	ArchitectPhoneNumber *string         `json:"architect_phone_number"`
	StandID              uuid.UUID       `json:"stand_id" validate:"required"`
	ApplicantID          string          `json:"applicant_id" validate:"required,uuid4"`
	AssignedGroupID      *uuid.UUID      `json:"assigned_group_id" validate:"required,uuid4"`
	TariffID             string          `json:"tariff_id" validate:"required,uuid4"`
	PropertyTypeID       string          `json:"property_type_id" validate:"required,uuid4"`
	DevelopmentLevy      decimal.Decimal `json:"development_levy" validate:"required,min=0"`
	VATAmount            decimal.Decimal `json:"vat_amount" validate:"required,min=0"`
	TotalCost            decimal.Decimal `json:"total_cost" validate:"required,min=0"`
	EstimatedCost        decimal.Decimal `json:"estimated_cost" validate:"required,min=0"`
	Status               string          `json:"status" validate:"required"`
	PaymentStatus        string          `json:"payment_status" validate:"required"`
	CreatedBy            string          `json:"created_by" validate:"required,email"`
}

// CreateApplication handles the creation of a new application
func (ac *ApplicationController) CreateApplicationController(c *fiber.Ctx) error {
	var req CreateApplicationRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Invalid request body for CreateApplication", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Start transaction
	config.Logger.Info("Starting transaction for application creation")
	tx := ac.DB.Session(&gorm.Session{}).WithContext(c.Context()).Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to start transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to start transaction",
			"error":   tx.Error.Error(),
		})
	}

	// Defer transaction rollback/commit handling
	txCommitted := false
	defer func() {
		if !txCommitted && tx != nil {
			tx.Rollback()
			config.Logger.Warn("Transaction rolled back due to error")
		}
	}()

	// Validate and parse submission date
	submissionDate := time.Now()

	// Verify applicant exists within transaction
	var applicant models.Applicant
	if err := tx.Where("id = ?", req.ApplicantID).First(&applicant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			config.Logger.Error("Applicant not found", zap.String("applicantID", req.ApplicantID))
			tx.Rollback()
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Applicant not found",
				"error":   "invalid_applicant",
			})
		}
		config.Logger.Error("Failed to verify applicant", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to verify applicant",
			"error":   err.Error(),
		})
	}

	// Verify tariff exists within transaction
	var tariff models.Tariff
	if err := tx.Preload("DevelopmentCategory").Where("id = ?", req.TariffID).First(&tariff).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			config.Logger.Error("Tariff not found", zap.String("tariffID", req.TariffID))
			tx.Rollback()
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Tariff not found",
				"error":   "invalid_tariff",
			})
		}
		config.Logger.Error("Failed to verify tariff", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to verify tariff",
			"error":   err.Error(),
		})
	}

	// Get active VAT rate within transaction
	activeVatRate, err := ac.getActiveVATRate(tx)
	if err != nil {
		config.Logger.Error("Failed to get active VAT rate", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get active VAT rate",
			"error":   err.Error(),
		})
	}

	if activeVatRate == nil {
		config.Logger.Error("No active VAT rate found")
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No active VAT rate available",
			"error":   "no_active_vat_rate",
		})
	}

	// Generate plan number
	planNumber, err := ac.generatePlanNumber(tx)
	if err != nil {
		config.Logger.Error("Failed to generate plan number", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate plan number",
			"error":   err.Error(),
		})
	}

	// Generate permit number
	permitNumber, err := ac.generatePermitNumber(tx)
	if err != nil {
		config.Logger.Error("Failed to generate permit number", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate permit number",
			"error":   err.Error(),
		})
	}

	// Prepare the new application for creation
	newApplication := models.Application{
		ID:                   uuid.New(),
		PlanNumber:           planNumber,
		PermitNumber:         permitNumber,
		PlanArea:             &req.PlanArea,
		ArchitectFullName:    req.ArchitectFullName,
		ArchitectEmail:       req.ArchitectEmail,
		ArchitectPhoneNumber: req.ArchitectPhoneNumber,
		DevelopmentLevy:      &req.DevelopmentLevy,
		VATAmount:            &req.VATAmount,
		TotalCost:            &req.TotalCost,
		EstimatedCost:        &req.EstimatedCost,
		PaymentStatus:        models.PaymentStatus(req.PaymentStatus),
		Status:               models.ApplicationStatus(req.Status),
		SubmissionDate:       submissionDate,
		StandID:              &req.StandID,
		AssignedGroupID:      req.AssignedGroupID,
		ApplicantID:          uuid.MustParse(req.ApplicantID),
		TariffID:             &tariff.ID,
		VATRateID:            &activeVatRate.ID,
		IsCollected:          false,
		CreatedBy:            req.CreatedBy,
	}

	// Set audit fields
	now := time.Now()
	newApplication.CreatedAt = now
	newApplication.UpdatedAt = now

	// Create the application within the transaction
	createdApplication, err := ac.createApplication(tx, &newApplication)
	if err != nil {
		config.Logger.Error("Failed to create application", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create application",
			"error":   err.Error(),
		})
	}

	// Generate quotation filename - remove slashes from plan number
	safePlanNumber := strings.ReplaceAll(createdApplication.PlanNumber, "/", "_")
	filename := fmt.Sprintf("quotation_%s_%s.pdf", safePlanNumber, time.Now().Format("20060102_150405"))

	// Generate the quotation PDF
	pdfPath, err := utils.GenerateDevelopmentPermitQuotation(*createdApplication, filename)
	if err != nil {
		config.Logger.Error("Failed to generate quotation PDF",
			zap.Error(err),
			zap.String("applicationID", createdApplication.ID.String()))

		// Rollback transaction since PDF generation failed
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Application created but quotation generation failed",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Quotation generated successfully",
		zap.String("pdfPath", pdfPath),
		zap.String("applicationID", createdApplication.ID.String()))

	// Read the generated PDF file
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		config.Logger.Error("Failed to read generated quotation PDF",
			zap.String("pdfPath", pdfPath),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read generated quotation PDF",
			"error":   err.Error(),
		})
	}

	// Validate PDF was actually generated and has content
	if len(pdfBytes) == 0 {
		config.Logger.Error("Generated quotation PDF is empty",
			zap.String("pdfPath", pdfPath))
		tx.Rollback()
		// Clean up empty PDF file
		os.Remove(pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Generated quotation PDF is empty",
			"error":   "empty_pdf",
		})
	}

	config.Logger.Info("Quotation PDF file loaded successfully",
		zap.String("path", pdfPath),
		zap.Int("size_bytes", len(pdfBytes)))

	// Create document request using the service pattern
	documentRequest := &documents_requests.CreateDocumentRequest{
		CategoryCode:   "DEVELOPMENT_PERMIT_QUOTATION", // Make sure this category exists
		FileName:       filename,
		ApplicationID:  &createdApplication.ID,
		ApplicantID:    &createdApplication.ApplicantID,
		CreatedBy:      req.CreatedBy,
		FileType:       "application/pdf",
	}

	// Create quotation document using DocumentService
	response, err := ac.DocumentSvc.UnifiedCreateDocument(
		tx,
		c,
		documentRequest,
		pdfBytes,
		nil, // No multipart file header since we're using bytes
	)
	if err != nil {
		config.Logger.Error("Failed to create quotation document",
			zap.String("applicationID", createdApplication.ID.String()),
			zap.Error(err))

		// Clean up the generated PDF file since document creation failed
		if cleanupErr := os.Remove(pdfPath); cleanupErr != nil {
			config.Logger.Warn("Failed to cleanup PDF file after document creation failure",
				zap.String("pdfPath", pdfPath),
				zap.Error(cleanupErr))
		}

		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create quotation document",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Quotation document created successfully",
		zap.String("documentID", response.ID.String()),
		zap.String("applicationID", createdApplication.ID.String()))

	// Preload the document for the response
	if err := tx.Preload("Documents").
		Preload("Applicant").
		Preload("Tariff").
		Preload("Stand").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		First(createdApplication, createdApplication.ID).Error; err != nil {
		config.Logger.Error("Failed to preload application relationships", zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to load application details",
			"error":   err.Error(),
		})
	}

	// Commit the transaction after all operations succeed
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize application creation",
			"error":   err.Error(),
		})
	}
	txCommitted = true
	config.Logger.Info("Transaction committed successfully")

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Application created successfully",
		"data": fiber.Map{
			"application": createdApplication,
			"quotation": fiber.Map{
				"document_id": response.ID,
				"filename":    filename,
				"file_path":   response.Document.FilePath,
				"generated_at": time.Now().Format(time.RFC3339),
			},
		},
	})
}

// Helper method to get active VAT rate within transaction
func (ac *ApplicationController) getActiveVATRate(tx *gorm.DB) (*models.VATRate, error) {
	var vatRate models.VATRate

	now := time.Now()
	err := tx.Where("is_active = ? AND (valid_to IS NULL OR valid_to > ?)", true, now).
		First(&vatRate).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &vatRate, nil
}

// Helper method to generate unique plan number
func (ac *ApplicationController) generatePlanNumber(tx *gorm.DB) (string, error) {
	// Get current year and month
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	// Count applications for this month
	var count int64
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

	if err := tx.Model(&models.Application{}).
		Where("created_at BETWEEN ? AND ?", startOfMonth, endOfMonth).
		Count(&count).Error; err != nil {
		return "", err
	}

	// Generate plan number: PLAN/YYYY/MM/XXX
	sequence := count + 1
	planNumber := fmt.Sprintf("PLAN/%d/%02d/%03d", year, month, sequence)

	return planNumber, nil
}

// Helper method to generate unique permit number
func (ac *ApplicationController) generatePermitNumber(tx *gorm.DB) (string, error) {
	// Get current year and month
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	// Count applications for this month
	var count int64
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

	if err := tx.Model(&models.Application{}).
		Where("created_at BETWEEN ? AND ?", startOfMonth, endOfMonth).
		Count(&count).Error; err != nil {
		return "", err
	}

	// Generate permit number: PERMIT/YYYY/MM/XXX
	sequence := count + 1
	permitNumber := fmt.Sprintf("PERMIT/%d/%02d/%03d", year, month, sequence)

	return permitNumber, nil
}

// Helper method to create application within transaction
func (ac *ApplicationController) createApplication(tx *gorm.DB, application *models.Application) (*models.Application, error) {
	if err := tx.Create(application).Error; err != nil {
		return nil, err
	}

	// Preload relationships for the response
	if err := tx.Preload("Applicant").
		Preload("Tariff").
		Preload("Stand").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		First(application, application.ID).Error; err != nil {
		return nil, err
	}

	return application, nil
}
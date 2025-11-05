package controllers

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	documents_requests "town-planning-backend/documents/requests"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type GenerateTPD1FormRequest struct {
	CreatedBy string `json:"created_by"`
}

// GenerateTPD1FormController handles the generation of TPD-1 form PDF
func (ac *ApplicationController) GenerateTPD1FormController(c *fiber.Ctx) error {
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

	var req GenerateTPD1FormRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Invalid request body for GenerateTPD1FormController", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   "invalid_request_body",
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

	// Start transaction for document creation
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
	defer func() {
		if !txCommitted && tx != nil {
			tx.Rollback()
			config.Logger.Warn("Transaction rolled back due to error")
		}
	}()

	// Get application with all required relationships
	var application models.Application
	if err := tx.
		Preload("Applicant").
		Preload("Tariff").
		Preload("Stand").
		Preload("Payment", "payment_for = ? AND payment_status = ?", "APPLICATION_FEE", "PAID").
		Preload("VATRate").
		First(&application, "id = ?", appUUID).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			config.Logger.Error("Application not found", zap.String("applicationID", applicationID))
			tx.Rollback()
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Application not found",
				"error":   "application_not_found",
			})
		}

		config.Logger.Error("Failed to fetch application",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch application details",
			"error":   err.Error(),
		})
	}

	// Validate that application has required data
	if err := validateApplicationForTPD1(application); err != nil {
		config.Logger.Error("Application data incomplete for TPD-1 form generation",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Application data incomplete for TPD-1 form generation",
			"error":   err.Error(),
		})
	}

	// Generate standardized filename
	filename := generateTPD1Filename(application)

	// Generate TPD-1 form PDF
	config.Logger.Info("Generating TPD-1 form PDF",
		zap.String("applicationID", applicationID),
		zap.String("filename", filename))

	pdfPath, err := utils.GenerateTPD1Form(application, filename)
	if err != nil {
		config.Logger.Error("Failed to generate TPD-1 form PDF",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate TPD-1 form PDF",
			"error":   err.Error(),
		})
	}

	// Read the generated PDF file
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		config.Logger.Error("Failed to read generated PDF",
			zap.String("pdfPath", pdfPath),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read generated PDF",
			"error":   err.Error(),
		})
	}

	// Validate PDF was actually generated and has content
	if len(pdfBytes) == 0 {
		config.Logger.Error("Generated PDF is empty",
			zap.String("pdfPath", pdfPath))
		tx.Rollback()
		// Clean up empty PDF file
		os.Remove(pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Generated PDF is empty",
			"error":   "empty_pdf",
		})
	}

	config.Logger.Info("PDF file loaded successfully",
		zap.String("path", pdfPath),
		zap.Int("size_bytes", len(pdfBytes)))

	// Create document request using the service pattern
	documentRequest := &documents_requests.CreateDocumentRequest{
		CategoryCode:   "TPD1_FORM", // Make sure this category exists in your categories table
		FileName:       filename,
		ApplicationID:  &appUUID,
		ApplicantID:    &application.Applicant.ID,
		CreatedBy:      req.CreatedBy,
		FileType:       "application/pdf",
	}

	// Create document using DocumentService (like the accommodation form example)
	response, err := ac.DocumentSvc.UnifiedCreateDocument(
		tx,
		c,
		documentRequest,
		pdfBytes,
		nil, // No multipart file header since we're using bytes
	)
	if err != nil {
		config.Logger.Error("Failed to create TPD-1 form document",
			zap.String("applicationID", applicationID),
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
			"message": "Failed to create TPD-1 form document",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("TPD-1 form document created successfully",
		zap.String("documentID", response.ID.String()),
		zap.String("applicationID", applicationID))

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize TPD-1 form generation",
			"error":   err.Error(),
		})
	}
	txCommitted = true

	config.Logger.Info("TPD-1 form generated successfully",
		zap.String("applicationID", applicationID),
		zap.String("pdfPath", pdfPath))

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "TPD-1 form generated successfully",
		"data": fiber.Map{
			"application_id": application.ID,
			"plan_number":    application.PlanNumber,
			"permit_number":  application.PermitNumber,
			"applicant_name": application.Applicant.FullName,
			"pdf_path":       response.Document.FilePath, // Use the path from the service response
			"filename":       filename,
			"document_id":    response.ID,
			"generated_at":   time.Now().Format(time.RFC3339),
		},
	})
}

// generateTPD1Filename generates a standardized filename for TPD-1 forms
func generateTPD1Filename(application models.Application) string {
	// Clean the applicant name for filename use
	cleanName := cleanStringForFilename(application.Applicant.FullName)

	// Get current timestamp in YYYYMMDD_HHMMSS format
	timestamp := time.Now().Format("20060102_150405")

	// Construct filename
	filename := fmt.Sprintf("%s_tpd1_form_%s.pdf", cleanName, timestamp)

	return filename
}

// cleanStringForFilename cleans a string for safe use in filenames
func cleanStringForFilename(input string) string {
	// Convert to lowercase
	clean := strings.ToLower(input)

	// Replace spaces and special characters with underscores
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, "-", "_")
	clean = strings.ReplaceAll(clean, ".", "_")

	// Remove any other non-alphanumeric characters except underscores
	reg := regexp.MustCompile(`[^a-z0-9_]`)
	clean = reg.ReplaceAllString(clean, "")

	// Remove multiple consecutive underscores
	reg = regexp.MustCompile(`_+`)
	clean = reg.ReplaceAllString(clean, "_")

	// Trim underscores from start and end
	clean = strings.Trim(clean, "_")

	// If the cleaned string is empty, use a fallback
	if clean == "" {
		clean = "applicant"
	}

	// Limit length to avoid filesystem issues (max 100 chars for name part)
	if len(clean) > 100 {
		clean = clean[:100]
	}

	return clean
}

// validateApplicationForTPD1 validates that the application has all required data for TPD-1 form generation
func validateApplicationForTPD1(application models.Application) error {
	// Check required fields
	if application.PlanNumber == "" {
		return fmt.Errorf("plan number is required")
	}
	if application.PermitNumber == "" {
		return fmt.Errorf("permit number is required")
	}
	if application.Applicant.ID == uuid.Nil {
		return fmt.Errorf("applicant information is required")
	}
	if application.Applicant.FullName == "" {
		return fmt.Errorf("applicant full name is required")
	}
	if application.Applicant.PostalAddress == nil || *application.Applicant.PostalAddress == "" {
		return fmt.Errorf("applicant postal address is required")
	}
	if application.Stand.StandNumber == "" {
		return fmt.Errorf("stand information is required")
	}
	if application.Tariff.ID == uuid.Nil {
		return fmt.Errorf("tariff information is required")
	}

	return nil
}
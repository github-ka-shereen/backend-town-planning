package controllers

import (
	"fmt"
	"os"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	documents_requests "town-planning-backend/documents/requests"
	"town-planning-backend/token"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type GenerateDevelopmentPermitRequest struct {
	CreatedBy string `json:"created_by"`
}

// GenerateDevelopmentPermitController handles the generation of Development Permit PDF
func (ac *ApplicationController) GenerateDevelopmentPermitController(c *fiber.Ctx) error {
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

	var req GenerateDevelopmentPermitRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Invalid request body for GenerateDevelopmentPermitController", zap.Error(err))
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

	// Get user from context (set by authentication middleware)
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	userUUID := payload.UserID

	user, err := ac.UserRepo.GetUserByID(userUUID.String())
	if err != nil {
		config.Logger.Error("Failed to get user by UUID", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get user by UUID",
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

	txCommitted := false
	defer func() {
		if !txCommitted && tx != nil {
			tx.Rollback()
			config.Logger.Warn("Transaction rolled back due to error")
		}
	}()

	// Get application with required relationships for development permit
	var application models.Application
	if err := tx.
		Preload("Applicant").
		Preload("Stand.StandType").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Preload("FinalApprover").
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

	// Get final approval decision
	var finalApproval models.FinalApproval
	if err := tx.
		Preload("Approver").
		Preload("Approver.Department").
		Where("application_id = ?", appUUID).
		First(&finalApproval).Error; err != nil && err != gorm.ErrRecordNotFound {
		config.Logger.Error("Failed to fetch final approval",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch final approval data",
			"error":   err.Error(),
		})
	}

	// Generate standardized filename
	filename := generateDevelopmentPermitFilename(application)

	// Generate Development Permit PDF
	config.Logger.Info("Generating Development Permit PDF",
		zap.String("applicationID", applicationID),
		zap.String("filename", filename))

	pdfPath, err := utils.GenerateDevelopmentPermit(application, finalApproval, filename, user)
	if err != nil {
		config.Logger.Error("Failed to generate Development Permit PDF",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate Development Permit PDF",
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

	// Validate PDF content
	if len(pdfBytes) == 0 {
		config.Logger.Error("Generated PDF is empty",
			zap.String("pdfPath", pdfPath))
		tx.Rollback()
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

	// Create document request
	documentRequest := &documents_requests.CreateDocumentRequest{
		CategoryCode:  "DEVELOPMENT_PERMIT",
		FileName:      filename,
		ApplicationID: &appUUID,
		ApplicantID:   &application.Applicant.ID,
		CreatedBy:     req.CreatedBy,
		FileType:      "application/pdf",
	}

	// Create document using DocumentService
	response, err := ac.DocumentSvc.UnifiedCreateDocument(
		tx,
		c,
		documentRequest,
		pdfBytes,
		nil,
	)
	if err != nil {
		config.Logger.Error("Failed to create Development Permit document",
			zap.String("applicationID", applicationID),
			zap.Error(err))

		if cleanupErr := os.Remove(pdfPath); cleanupErr != nil {
			config.Logger.Warn("Failed to cleanup PDF file after document creation failure",
				zap.String("pdfPath", pdfPath),
				zap.Error(cleanupErr))
		}

		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create Development Permit document",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Development Permit document created successfully",
		zap.String("documentID", response.ID.String()),
		zap.String("applicationID", applicationID))

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize Development Permit generation",
			"error":   err.Error(),
		})
	}
	txCommitted = true

	config.Logger.Info("Development Permit generated successfully",
		zap.String("applicationID", applicationID),
		zap.String("pdfPath", pdfPath))

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Development Permit generated successfully",
		"data": fiber.Map{
			"application_id": application.ID,
			"plan_number":    application.PlanNumber,
			"permit_number":  application.PermitNumber,
			"applicant_name": application.Applicant.FullName,
			"pdf_path":       response.Document.FilePath,
			"filename":       filename,
			"document_id":    response.ID,
			"generated_at":   time.Now().Format(time.RFC3339),
			"generated_by":   user.FirstName + " " + user.LastName,
		},
	})
}

// generateDevelopmentPermitFilename generates a standardized filename
func generateDevelopmentPermitFilename(application models.Application) string {
	cleanName := cleanStringForFilename(application.Applicant.FullName)
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("DEVELOPMENT_PERMIT_%s_%s.pdf", cleanName, timestamp)
	return filename
}

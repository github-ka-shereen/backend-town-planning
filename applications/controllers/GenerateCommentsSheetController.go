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

type GenerateCommentsSheetRequest struct {
	CreatedBy string `json:"created_by"`
}

// GenerateCommentsSheetController handles the generation of Plan Comments Sheet PDF
func (ac *ApplicationController) GenerateCommentsSheetController(c *fiber.Ctx) error {
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

	var req GenerateCommentsSheetRequest
	if err := c.BodyParser(&req); err != nil {
		config.Logger.Error("Invalid request body for GenerateCommentsSheetController", zap.Error(err))
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

	// Get application with required relationships
	var application models.Application
	if err := tx.
		Preload("Applicant").
		Preload("Stand.StandType").
		Preload("Tariff.DevelopmentCategory").
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

	// Get group assignments with final comments
	var assignments []models.ApplicationGroupAssignment
	if err := tx.
		Preload("Group").
		Preload("Decisions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Decisions.Member").
		Preload("Decisions.User.Department").
		Preload("Decisions.Comments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Where("application_id = ? AND is_active = ?", appUUID, true).
		Find(&assignments).Error; err != nil {

		config.Logger.Error("Failed to fetch approval assignments",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch approval data",
			"error":   err.Error(),
		})
	}

	// Extract final comments from decisions
	finalComments := extractFinalCommentsFromDecisions(assignments)

	if len(finalComments) == 0 {
		config.Logger.Warn("No comments found for application",
			zap.String("applicationID", applicationID))
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No comments found for this application",
			"error":   "no_comments_found",
		})
	}

	// Generate standardized filename
	filename := generateCommentsSheetFilename(application)

	// Generate Comments Sheet PDF
	config.Logger.Info("Generating Plan Comments Sheet PDF",
		zap.String("applicationID", applicationID),
		zap.String("filename", filename))

	pdfPath, err := utils.GenerateCommentsSheet(application, finalComments, filename, user)
	if err != nil {
		config.Logger.Error("Failed to generate Comments Sheet PDF",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate Comments Sheet PDF",
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
		CategoryCode:  "PLAN_COMMENTS_SHEET",
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
		config.Logger.Error("Failed to create Comments Sheet document",
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
			"message": "Failed to create Comments Sheet document",
			"error":   err.Error(),
		})
	}

	config.Logger.Info("Comments Sheet document created successfully",
		zap.String("documentID", response.ID.String()),
		zap.String("applicationID", applicationID))

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to finalize Comments Sheet generation",
			"error":   err.Error(),
		})
	}
	txCommitted = true

	config.Logger.Info("Comments Sheet generated successfully",
		zap.String("applicationID", applicationID),
		zap.String("pdfPath", pdfPath))

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Plan Comments Sheet generated successfully",
		"data": fiber.Map{
			"application_id": application.ID,
			"plan_number":    application.PlanNumber,
			"applicant_name": application.Applicant.FullName,
			"pdf_path":       response.Document.FilePath,
			"filename":       filename,
			"document_id":    response.ID,
			"comments_count": len(finalComments),
			"generated_at":   time.Now().Format(time.RFC3339),
		},
	})
}

// generateCommentsSheetFilename generates a standardized filename
func generateCommentsSheetFilename(application models.Application) string {
	cleanName := cleanStringForFilename(application.Applicant.FullName)
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_plan_comments_%s.pdf", cleanName, timestamp)
	return filename
}

// extractFinalCommentsFromDecisions extracts the final comment from each decision
func extractFinalCommentsFromDecisions(assignments []models.ApplicationGroupAssignment) []utils.FinalComment {
	var finalComments []utils.FinalComment
	seenDecisions := make(map[uuid.UUID]bool)

	for _, assignment := range assignments {
		for _, decision := range assignment.Decisions {
			// Skip if we've already processed this decision
			if seenDecisions[decision.ID] {
				continue
			}
			seenDecisions[decision.ID] = true

			// Get the latest comment for this decision
			if len(decision.Comments) > 0 {
				// Find the latest comment (they're already ordered DESC)
				latestComment := decision.Comments[0]
				for _, comment := range decision.Comments {
					if comment.CreatedAt.After(latestComment.CreatedAt) {
						latestComment = comment
					}
				}

				// Get department name safely
				departmentName := "Unknown"
				if decision.User.Department != nil && decision.User.Department.Name != "" {
					departmentName = decision.User.Department.Name
				}

				// Get signature path safely
				signaturePath := ""
				if decision.User.SignatureFilePath != nil {
					signaturePath = *decision.User.SignatureFilePath
				}

				finalComment := utils.FinalComment{
					SectionName:      departmentName,
					Date:             decision.DecidedAt,
					Status:           string(decision.Status),
					ReviewerName:     fmt.Sprintf("%s %s", decision.User.FirstName, decision.User.LastName),
					Comment:          latestComment.Content,
					CommentCreatedAt: latestComment.CreatedAt,
					DepartmentName:   departmentName,
					SignaturePath:    signaturePath,
				}

				finalComments = append(finalComments, finalComment)
			}
		}
	}

	return finalComments
}

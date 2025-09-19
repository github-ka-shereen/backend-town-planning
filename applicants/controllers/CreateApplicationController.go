package controllers

import (
	"context"
	"town-planning-backend/applicants/repositories"
	"town-planning-backend/applicants/services"
	indexing_repository "town-planning-backend/bleve/repositories"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	// documents_services "town-planning-backend/documents/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ApplicantController struct {
	ApplicantRepo      repositories.ApplicantRepository
	DB                 *gorm.DB
	Ctx                context.Context
	BleveRepo          indexing_repository.BleveRepositoryInterface
	// DocumentSvc        *documents_services.DocumentService
	// ApplicationService services.ApplicationService
}

func (ac *ApplicantController) CreateApplicantController(c *fiber.Ctx) error {
	var applicant models.Applicant

	// Parse incoming JSON payload
	if err := c.BodyParser(&applicant); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request payload",
			"error":   err.Error(),
		})
	}

	// Validate applicant type
	if applicant.ApplicantType != models.IndividualApplicant && applicant.ApplicantType != models.OrganisationApplicant {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid applicant type",
		})
	}

	// Validate the client data
	validationError := services.ValidateApplicant(&applicant)
	if validationError != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation failed",
			"error":   validationError,
		})
	}

	// Check if there are additional phone numbers and set their created_by to the client's created_by
	if len(applicant.AdditionalPhoneNumbers) > 0 {
		for i := range applicant.AdditionalPhoneNumbers {
			applicant.AdditionalPhoneNumbers[i].CreatedBy = applicant.CreatedBy
		}
	}

	// Check if there are company representatives and set their created_by to the client's created_by
	if len(applicant.OrganisationRepresentatives) > 0 {
		for i := range applicant.OrganisationRepresentatives {
			applicant.OrganisationRepresentatives[i].CreatedBy = applicant.CreatedBy
		}
	}

	// --- Start Database Transaction ---
	tx := ac.DB.Begin()
	if tx.Error != nil {
		config.Logger.Error("Failed to begin database transaction", zap.Error(tx.Error))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not start database transaction",
			"error":   tx.Error.Error(),
		})
	}

	// Defer rollback in case of error or panic
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic detected, rolling back transaction", zap.Any("panic_reason", r))
			panic(r)
		}
		if tx.Error != nil {
			config.Logger.Warn("Transaction not committed, attempting rollback due to prior error", zap.Error(tx.Error))
			tx.Rollback()
		}
	}()


	// Save the applicant to the database
	createdApplicant, err := ac.ApplicantRepo.CreateApplicant(tx, &applicant)
	if err != nil {
		// The defer will handle the rollback here.
		config.Logger.Error("Failed to create applicant in database", zap.Error(err), zap.String("applicantName", applicant.FullName))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Something went wrong while creating user in the database",
			"data":    nil,
			"error":   err.Error(),
		})
	}

	// --- Bleve Indexing ---
	if ac.BleveRepo != nil {
		err := ac.BleveRepo.IndexSingleApplicant(*createdApplicant)
		if err != nil {
			// Explicit rollback
			tx.Rollback()
			config.Logger.Error(
				"CRITICAL: Failed to index applicant in Bleve. Rolling back database transaction.",
				zap.Error(err),
				zap.String("applicantID", createdApplicant.ID.String()),
				zap.String("applicantName", createdApplicant.FullName),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Client could not be created because indexing failed. Please try again or contact support.",
				"error":   err.Error(),
			})
		} else {
			config.Logger.Info("Successfully indexed applicant in Bleve", zap.String("applicantID", createdApplicant.ID.String()))
		}
	} else {
		// Explicit rollback for missing IndexingService
		tx.Rollback()
		config.Logger.Error(
			"IndexingService is nil, cannot index applicant. Rolling back transaction.",
			zap.String("applicantID", createdApplicant.ID.String()),
			zap.String("applicantName", createdApplicant.FullName),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Server configuration error: Indexing service not available. Cannot create applicant.",
			"error":   "indexing_service_not_configured",
		})
	}

	// --- Commit Database Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	// Return the created client
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Client successfully created",
		"data":    createdApplicant,
	})
}

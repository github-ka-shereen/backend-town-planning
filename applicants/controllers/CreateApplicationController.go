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
	ApplicantRepo repositories.ApplicantRepository
	DB            *gorm.DB
	Ctx           context.Context
	BleveRepo     indexing_repository.BleveRepositoryInterface
	// DocumentSvc        *documents_services.DocumentService
	// ApplicationService services.ApplicationService
}

type CreateApplicantRequest struct {
	ApplicantType                   models.ApplicantType                `json:"applicant_type"`
	FirstName                       *string                             `json:"first_name"`
	LastName                        *string                             `json:"last_name"`
	MiddleName                      *string                             `json:"middle_name"`
	OrganisationName                *string                             `json:"organisation_name"`
	TaxIdentificationNumber         *string                             `json:"tax_identification_number"`
	IdNumber                        *string                             `json:"id_number"`
	Email                           string                              `json:"email"`
	PhoneNumber                     string                              `json:"phone_number"`
	WhatsAppNumber                  string                              `json:"whatsapp_number"`
	PostalAddress                   *string                             `json:"postal_address"`
	City                            *string                             `json:"city"`
	Gender                          *string                             `json:"gender"`
	CreatedBy                       string                              `json:"created_by"`
	OrganisationRepresentatives     []OrganisationRepresentativeRequest `json:"organisation_representatives"`
	ApplicantAdditionalPhoneNumbers []AdditionalPhoneRequest            `json:"applicant_additional_phone_numbers"`
}

type OrganisationRepresentativeRequest struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Role        string `json:"role"`
}

type AdditionalPhoneRequest struct {
	PhoneNumber string `json:"phone_number"`
}

func (ac *ApplicantController) CreateApplicantController(c *fiber.Ctx) error {
	var request CreateApplicantRequest

	// Parse incoming JSON payload
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request payload",
			"error":   err.Error(),
		})
	}

	// Map DTO to GORM model
	applicant := models.Applicant{
		ApplicantType:           request.ApplicantType,
		FirstName:               request.FirstName,
		LastName:                request.LastName,
		MiddleName:              request.MiddleName,
		OrganisationName:        request.OrganisationName,
		TaxIdentificationNumber: request.TaxIdentificationNumber,
		IdNumber:                request.IdNumber,
		Email:                   request.Email,
		PhoneNumber:             request.PhoneNumber,
		WhatsAppNumber:          &request.WhatsAppNumber,
		PostalAddress:           request.PostalAddress,
		City:                    request.City,
		Gender:                  request.Gender,
		CreatedBy:               request.CreatedBy,
		Status:                  models.ProspectiveApplicant, // Set default status
	}

	// Map organisation representatives
	for _, repReq := range request.OrganisationRepresentatives {
		rep := models.OrganisationRepresentative{
			FirstName:   repReq.FirstName,
			LastName:    repReq.LastName,
			Email:       repReq.Email,
			PhoneNumber: repReq.PhoneNumber,
			Role:        repReq.Role,
			CreatedBy:   request.CreatedBy,
		}
		applicant.OrganisationRepresentatives = append(applicant.OrganisationRepresentatives, rep)
	}

	// Map additional phone numbers
	for _, phoneReq := range request.ApplicantAdditionalPhoneNumbers {
		phone := models.ApplicantAdditionalPhone{
			PhoneNumber: phoneReq.PhoneNumber,
			CreatedBy:   request.CreatedBy,
		}
		applicant.AdditionalPhoneNumbers = append(applicant.AdditionalPhoneNumbers, phone)
	}

	// Validate applicant type
	if applicant.ApplicantType != models.IndividualApplicant && applicant.ApplicantType != models.OrganisationApplicant {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid applicant type",
		})
	}

	// Validate the applicant data
	validationError := services.ValidateApplicant(&applicant)
	if validationError != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation failed",
			"error":   validationError,
		})
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

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			config.Logger.Error("Panic detected, rolling back transaction", zap.Any("panic_reason", r))
			panic(r)
		}
	}()

	// Save the applicant to the database
	createdApplicant, err := ac.ApplicantRepo.CreateApplicant(tx, &applicant)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to create applicant in database", zap.Error(err), zap.String("applicantName", applicant.FullName))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Something went wrong while creating applicant in the database",
			"error":   err.Error(),
		})
	}

	// --- Bleve Indexing ---
	if ac.BleveRepo != nil {
		err := ac.BleveRepo.IndexSingleApplicant(*createdApplicant)
		if err != nil {
			tx.Rollback()
			config.Logger.Error(
				"Failed to index applicant in Bleve. Rolling back database transaction.",
				zap.Error(err),
				zap.String("applicantID", createdApplicant.ID.String()),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Applicant could not be created because indexing failed",
				"error":   err.Error(),
			})
		}
		config.Logger.Info("Successfully indexed applicant in Bleve", zap.String("applicantID", createdApplicant.ID.String()))
	}

	// --- Commit Database Transaction ---
	if err := tx.Commit().Error; err != nil {
		config.Logger.Error("Failed to commit database transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Internal server error: Could not commit database transaction",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Applicant successfully created",
		"data":    createdApplicant,
	})
}

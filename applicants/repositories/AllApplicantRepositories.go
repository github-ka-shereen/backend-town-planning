package repositories

import (
	"errors"
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ApplicantRepository interface {
	CreateApplicant(tx *gorm.DB, applicant *models.Applicant) (*models.Applicant, error)
}

type applicantRepository struct {
	DB *gorm.DB
}

// NewApplicantRepository initializes a new applicant repository
func NewApplicantRepository(db *gorm.DB) ApplicantRepository {
	return &applicantRepository{DB: db}
}

func (ar *applicantRepository) CreateApplicant(tx *gorm.DB, applicant *models.Applicant) (*models.Applicant, error) {
	// Set full name based on applicant type
	switch applicant.ApplicantType {
	case models.IndividualApplicant:
		applicant.FullName = applicant.GetFullName()
		config.Logger.Info("Set full name for individual applicant",
			zap.String("fullName", applicant.FullName))

	case models.OrganisationApplicant:
		if applicant.OrganisationName == nil {
			return nil, errors.New("organisation name is required for organisation applicants")
		}
		applicant.FullName = *applicant.OrganisationName
		config.Logger.Info("Set full name for organisation applicant",
			zap.String("organisationName", *applicant.OrganisationName))
	}

	// Set default status if not provided
	if applicant.Status == "" {
		applicant.Status = models.ProspectiveApplicant
		config.Logger.Info("Applicant status set to default",
			zap.String("status", string(applicant.Status)))
	}

	// Create the applicant
	if err := tx.Create(applicant).Error; err != nil {
		config.Logger.Error("Failed to create applicant",
			zap.Error(err),
			zap.Any("applicantData", applicant))
		return nil, fmt.Errorf("failed to create applicant: %w", err)
	}

	config.Logger.Info("Created applicant successfully",
		zap.String("applicantID", applicant.ID.String()))

	return applicant, nil
}

package repositories

import (
	"errors"
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ApplicantRepository interface {
	CreateApplicant(tx *gorm.DB, applicant *models.Applicant) (*models.Applicant, error)
	GetAllApplicants() ([]models.Applicant, error)
	GetFilteredApplicants(limit, offset int) ([]models.Applicant, int64, error)
	GetActiveVATRate(tx *gorm.DB) (*models.VATRate, error)
	DeactivateVATRate(tx *gorm.DB, vatRateID uuid.UUID, createdBy string) (*models.VATRate, error)
	CreateVATRate(tx *gorm.DB, vatRate *models.VATRate) (*models.VATRate, error)
	GetFilteredVatRates(limit, offset int, filters map[string]string) ([]models.VATRate, int64, error)
}

type applicantRepository struct {
	DB *gorm.DB
}

// NewApplicantRepository initializes a new applicant repository
func NewApplicantRepository(db *gorm.DB) ApplicantRepository {
	return &applicantRepository{DB: db}
}

func (ar *applicantRepository) GetAllApplicants() ([]models.Applicant, error) {
	var applicants []models.Applicant
	if err := ar.DB.Find(&applicants).Error; err != nil {
		config.Logger.Error("Failed to get all applicants", zap.Error(err))
		return nil, fmt.Errorf("failed to get all applicants: %w", err)
	}
	return applicants, nil
}

func (ar *applicantRepository) GetFilteredApplicants(limit, offset int) ([]models.Applicant, int64, error) {
	var applicants []models.Applicant
	var total int64

	// Count total number of applicants
	if err := ar.DB.Model(&models.Applicant{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated applicants, ordered by UpdatedAt and CreatedAt (descending)
	if err := ar.DB.Order("updated_at DESC, created_at DESC").Limit(limit).Offset(offset).Find(&applicants).Error; err != nil {
		return nil, 0, err
	}

	return applicants, total, nil
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

	// Create the applicant with associations
	if err := tx.Create(applicant).Error; err != nil {
		config.Logger.Error("Failed to create applicant",
			zap.Error(err),
			zap.Any("applicantData", applicant))
		return nil, fmt.Errorf("failed to create applicant: %w", err)
	}

	// If you need to load the relationships after creation:
	if err := tx.Preload("OrganisationRepresentatives").
		Preload("AdditionalPhoneNumbers").
		First(applicant, applicant.ID).Error; err != nil {
		config.Logger.Error("Failed to load applicant relationships",
			zap.Error(err),
			zap.String("applicantID", applicant.ID.String()))
		return nil, fmt.Errorf("failed to load applicant relationships: %w", err)
	}

	config.Logger.Info("Created applicant successfully",
		zap.String("applicantID", applicant.ID.String()),
		zap.Int("representatives", len(applicant.OrganisationRepresentatives)),
		zap.Int("phoneNumbers", len(applicant.AdditionalPhoneNumbers)))

	return applicant, nil
}

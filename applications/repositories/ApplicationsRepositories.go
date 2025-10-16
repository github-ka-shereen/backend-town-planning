// repositories/application_repository.go
package repositories

import (
	"strings"
	"time"
	"town-planning-backend/db/models"

	"gorm.io/gorm"
)

type ApplicationRepository interface {
	CreateDevelopmentCategory(category *models.DevelopmentCategory) (*models.DevelopmentCategory, error)
	GetDevelopmentCategoryByName(name string) (*models.DevelopmentCategory, error)
	GetFilteredDevelopmentCategories(pageSize, offset int, filters map[string]string) ([]models.DevelopmentCategory, int64, error)
	GetAllDevelopmentCategories(isActive *bool) ([]models.DevelopmentCategory, error)

	// Tariff methods (optional - you can use the controller helpers instead)
	CreateTariff(tariff *models.Tariff) (*models.Tariff, error)
	GetActiveTariffForCategory(developmentCategoryID string) (*models.Tariff, error)
	DeactivateTariff(tariffID string, updatedBy string) (*models.Tariff, error)
	GetFilteredDevelopmentTariffs(limit, offset int, filters map[string]string) ([]models.Tariff, int64, error)
	GetTariffByID(tariffID string) (*models.Tariff, error)
	GetFilteredApplications(limit, offset int, filters map[string]string) ([]models.Application, int64, error)
	GetApplicationById(applicationID string) (*models.Application, error)
}

type applicationRepository struct {
	db *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) ApplicationRepository {
	return &applicationRepository{db: db}
}

func (r *applicationRepository) GetApplicationById(applicationID string) (*models.Application, error) {
    var application models.Application
    if err := r.db.
        Preload("Applicant").
        Preload("Tariff").
        Preload("Tariff.DevelopmentCategory").
        Preload("VATRate").
        Where("id = ?", applicationID).
        First(&application).Error; err != nil {
        return nil, err
    }
    return &application, nil
}

// GetFilteredApplications fetches applications with filtering and pagination
func (r *applicationRepository) GetFilteredApplications(limit, offset int, filters map[string]string) ([]models.Application, int64, error) {
	var applications []models.Application
	var total int64

	// Start building the query with preloads
	query := r.db.Model(&models.Application{}).
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate")

	// Apply filters
	if applicantID, exists := filters["applicant_id"]; exists && applicantID != "" {
		query = query.Where("applicant_id = ?", applicantID)
	}

	if planNumber, exists := filters["plan_number"]; exists && planNumber != "" {
		query = query.Where("plan_number ILIKE ?", "%"+planNumber+"%")
	}

	if permitNumber, exists := filters["permit_number"]; exists && permitNumber != "" {
		query = query.Where("permit_number ILIKE ?", "%"+permitNumber+"%")
	}

	if status, exists := filters["status"]; exists && status != "" {
		query = query.Where("status = ?", status)
	}

	if paymentStatus, exists := filters["payment_status"]; exists && paymentStatus != "" {
		query = query.Where("payment_status = ?", paymentStatus)
	}

	if standID, exists := filters["stand_id"]; exists && standID != "" {
		query = query.Where("stand_id = ?", standID)
	}

	if architectName, exists := filters["architect_name"]; exists && architectName != "" {
		query = query.Where("architect_full_name ILIKE ?", "%"+architectName+"%")
	}

	if dateFrom, exists := filters["date_from"]; exists && dateFrom != "" {
		parsedDate, err := time.Parse("2006-01-02", dateFrom)
		if err == nil {
			query = query.Where("submission_date >= ?", parsedDate)
		}
	}

	if dateTo, exists := filters["date_to"]; exists && dateTo != "" {
		parsedDate, err := time.Parse("2006-01-02", dateTo)
		if err == nil {
			// Add one day to include the entire end date
			parsedDate = parsedDate.Add(24 * time.Hour)
			query = query.Where("submission_date < ?", parsedDate)
		}
	}

	if isCollected, exists := filters["is_collected"]; exists && isCollected != "" {
		if isCollected == "true" {
			query = query.Where("is_collected = ?", true)
		} else if isCollected == "false" {
			query = query.Where("is_collected = ?", false)
		}
	}

	// Count total number of records matching the filters
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated applications, ordered by submission date (descending) to show latest first
	if err := query.
		Order("submission_date DESC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&applications).Error; err != nil {
		return nil, 0, err
	}

	return applications, total, nil
}

// GetTariffByID fetches a tariff by ID
func (r *applicationRepository) GetTariffByID(tariffID string) (*models.Tariff, error) {
	var tariff models.Tariff
	if err := r.db.Preload("DevelopmentCategory").First(&tariff, tariffID).Error; err != nil {
		return nil, err
	}
	return &tariff, nil
}

// GetFilteredDevelopmentTariffs fetches tariffs with filtering and pagination
func (r *applicationRepository) GetFilteredDevelopmentTariffs(limit, offset int, filters map[string]string) ([]models.Tariff, int64, error) {
	var tariffs []models.Tariff
	var total int64

	// Start building the query
	query := r.db.Model(&models.Tariff{}).Preload("DevelopmentCategory")

	// Apply filters
	if developmentCategoryID, exists := filters["development_category_id"]; exists && developmentCategoryID != "" {
		query = query.Where("development_category_id = ?", developmentCategoryID)
	}

	if isActive, exists := filters["is_active"]; exists && isActive != "" {
		if isActive == "true" {
			query = query.Where("is_active = ?", true)
		} else if isActive == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	// Count total number of records matching the filters
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated tariffs, ordered by ValidFrom (descending) to show latest first
	if err := query.
		Order("valid_from DESC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&tariffs).Error; err != nil {
		return nil, 0, err
	}

	return tariffs, total, nil
}

// CreateDevelopmentCategory creates a new development category
func (r *applicationRepository) CreateDevelopmentCategory(category *models.DevelopmentCategory) (*models.DevelopmentCategory, error) {
	if err := r.db.Create(category).Error; err != nil {
		return nil, err
	}
	return category, nil
}

// GetDevelopmentCategoryByName finds a development category by name
func (r *applicationRepository) GetDevelopmentCategoryByName(name string) (*models.DevelopmentCategory, error) {
	var category models.DevelopmentCategory

	cleanName := strings.ToUpper(strings.TrimSpace(name))

	if err := r.db.Where("UPPER(TRIM(name)) = ? AND is_active = ?", cleanName, true).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

// CreateTariff creates a new tariff
func (r *applicationRepository) CreateTariff(tariff *models.Tariff) (*models.Tariff, error) {
	if err := r.db.Create(tariff).Error; err != nil {
		return nil, err
	}

	// Preload the development category relationship
	if err := r.db.Preload("DevelopmentCategory").First(tariff, tariff.ID).Error; err != nil {
		return nil, err
	}

	return tariff, nil
}

// GetActiveTariffForCategory gets the currently active tariff for a category
func (r *applicationRepository) GetActiveTariffForCategory(developmentCategoryID string) (*models.Tariff, error) {
	var tariff models.Tariff

	now := time.Now()
	err := r.db.Where("development_category_id = ? AND is_active = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)",
		developmentCategoryID, true, now, now).
		Order("valid_from DESC").
		First(&tariff).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &tariff, nil
}

// DeactivateTariff deactivates a tariff
func (r *applicationRepository) DeactivateTariff(tariffID string, updatedBy string) (*models.Tariff, error) {
	var tariff models.Tariff

	if err := r.db.Where("id = ?", tariffID).First(&tariff).Error; err != nil {
		return nil, err
	}

	tariff.IsActive = false
	tariff.UpdatedAt = time.Now()

	if err := r.db.Save(&tariff).Error; err != nil {
		return nil, err
	}

	// Preload the development category for the response
	if err := r.db.Preload("DevelopmentCategory").First(&tariff, tariff.ID).Error; err != nil {
		return nil, err
	}

	return &tariff, nil
}

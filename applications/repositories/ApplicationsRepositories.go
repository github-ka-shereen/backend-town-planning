package repositories

import (
	"strings"
	"town-planning-backend/db/models"

	"gorm.io/gorm"
)

type ApplicationRepository interface {
	CreateDevelopmentCategory(category *models.DevelopmentCategory) (*models.DevelopmentCategory, error)
	GetDevelopmentCategoryByName(name string) (*models.DevelopmentCategory, error)
}

type applicationRepository struct {
	DB *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) ApplicationRepository {
	return &applicationRepository{DB: db}
}

// CreateDevelopmentCategory creates a new development category
func (r *applicationRepository) CreateDevelopmentCategory(category *models.DevelopmentCategory) (*models.DevelopmentCategory, error) {
	if err := r.DB.Create(category).Error; err != nil {
		return nil, err
	}
	return category, nil
}

// GetDevelopmentCategoryByName finds a development category by name
func (r *applicationRepository) GetDevelopmentCategoryByName(name string) (*models.DevelopmentCategory, error) {
	var category models.DevelopmentCategory

	cleanName := strings.ToUpper(strings.TrimSpace(name))

	if err := r.DB.Where("UPPER(TRIM(name)) = ? AND is_active = ?", cleanName, true).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

package repositories

import (
	"errors"
	"fmt"
	"strings"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StandRepository interface {
	AddStandTypes(tx *gorm.DB, standType *models.StandType) (*models.StandType, error)
	GetFilteredStandTypes(pageSize int, offset int, filters map[string]string) ([]models.StandType, int64, error)
	GetProjectByProjectNumber(projectNumber string) (*models.Project, error)
	CreateProject(project *models.Project) (*models.Project, error)
}

type standRepository struct {
	db *gorm.DB
}

func NewStandRepository(db *gorm.DB) StandRepository {
	return &standRepository{
		db: db,
	}
}

func (r *standRepository) CreateProject(project *models.Project) (*models.Project, error) {
	project.ID = uuid.New()
	err := r.db.Create(project).Error
	return project, err
}

func (r *standRepository) GetProjectByProjectNumber(projectNumber string) (*models.Project, error) {
	var project models.Project
	err := r.db.First(&project, "project_number = ?", projectNumber).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return nil with a descriptive error instead of just nil, nil
			return nil, fmt.Errorf("project with project number '%s' not found", projectNumber)
		}
		return nil, err // Other database errors
	}
	return &project, nil // Project found
}

// AddStandTypes creates a new stand type in the database
func (r *standRepository) AddStandTypes(tx *gorm.DB, standType *models.StandType) (*models.StandType, error) {
	if err := tx.Create(standType).Error; err != nil {
		return nil, err
	}
	return standType, nil
}

// GetFilteredStandTypes retrieves stand types with filtering and pagination
func (r *standRepository) GetFilteredStandTypes(pageSize int, offset int, filters map[string]string) ([]models.StandType, int64, error) {
	var standTypes []models.StandType
	var total int64

	db := r.db.Model(&models.StandType{}) // start a new query chain

	// Apply filters
	for key, value := range filters {
		switch key {
		case "active":
			if strings.ToLower(value) == "true" {
				db = db.Where("is_active = ?", true)
			} else if strings.ToLower(value) == "false" {
				db = db.Where("is_active = ?", false)
			}
		case "start_date":
			db = db.Where("Date(created_at) >= ?", value)
		case "end_date":
			db = db.Where("Date(created_at) <= ?", value)
		case "name":
			db = db.Where("name ILIKE ?", "%"+value+"%")
		case "created_by":
			db = db.Where("created_by ILIKE ?", "%"+value+"%")
		case "is_system":
			if strings.ToLower(value) == "true" {
				db = db.Where("is_system = ?", true)
			} else if strings.ToLower(value) == "false" {
				db = db.Where("is_system = ?", false)
			}
		}
	}

	// Count total records with filters applied
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	if err := db.Limit(pageSize).Offset(offset).Order("created_at DESC").Find(&standTypes).Error; err != nil {
		return nil, 0, err
	}

	return standTypes, total, nil
}

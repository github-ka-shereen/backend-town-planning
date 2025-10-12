package repositories

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StandRepository interface {
	AddStandTypes(tx *gorm.DB, standType *models.StandType) (*models.StandType, error)
	GetFilteredStandTypes(pageSize int, offset int, filters map[string]string) ([]models.StandType, int64, error)
	GetProjectByProjectNumber(projectNumber string) (*models.Project, error)
	CreateProject(project *models.Project) (*models.Project, error)
	GetAllProjects() ([]models.Project, error)
	LogBulkUploadErrors(errors []models.BulkUploadErrorProjects) error
	LogDuplicateProjects(errors []models.BulkUploadErrorProjects) error
	LogEmailSent(emailLog *models.EmailLog) error
	FindDuplicateProjectNumbers(projectNumbers []string) ([]string, error)
	BulkCreateProjects(tx *gorm.DB, projects []models.Project) error
	GetFilteredProjects(projectName, city, startDate, endDate string, pageSize, page int) ([]models.Project, int64, error)
}

type standRepository struct {
	db *gorm.DB
}

func NewStandRepository(db *gorm.DB) StandRepository {
	return &standRepository{
		db: db,
	}
}

func (r *standRepository) FindDuplicateProjectNumbers(projectNumbers []string) ([]string, error) {
	var duplicates []string
	err := r.db.Model(&models.Project{}).
		Where("project_number IN ?", projectNumbers).
		Pluck("project_number", &duplicates).Error
	return duplicates, err
}

func (r *standRepository) LogEmailSent(emailLog *models.EmailLog) error {
	return r.db.Create(emailLog).Error
}

// BulkCreateProjects inserts multiple projects in one go
func (r *standRepository) BulkCreateProjects(tx *gorm.DB, projects []models.Project) error {
	if len(projects) == 0 {
		return nil
	}

	// Adding UUID for each project before batch insertion
	for i := range projects {
		projects[i].ID = uuid.New()
	}

	return tx.CreateInBatches(projects, 100).Error // Batch size of 100 (adjust as necessary)
}

// In the projectRepository struct implementation
func (r *standRepository) LogBulkUploadErrors(errors []models.BulkUploadErrorProjects) error {
	if len(errors) == 0 {
		return nil
	}
	return r.db.CreateInBatches(errors, 500).Error // Batch insertion of errors
}

func (r *standRepository) LogDuplicateProjects(errors []models.BulkUploadErrorProjects) error {
	if len(errors) == 0 {
		return nil
	}
	return r.db.CreateInBatches(errors, 500).Error // Batch insertion of errors
}

func (r *standRepository) GetAllDuplicates() ([]models.BulkUploadErrorProjects, error) {
	var duplicates []models.BulkUploadErrorProjects
	err := r.db.Find(&duplicates).Error
	return duplicates, err
}

func (r *standRepository) MarkDuplicateAsResolved(id string) error {
	return r.db.Model(&models.BulkUploadErrorProjects{}).
		Where("id = ?", id).
		Update("resolved", true).Error
}

func (r *standRepository) GetAllProjects() ([]models.Project, error) {
	var projects []models.Project
	err := r.db.Find(&projects).Error
	return projects, err
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

func (r *standRepository) GetFilteredProjects(projectName, city, startDate, endDate string, pageSize, page int) ([]models.Project, int64, error) {
	var projects []models.Project
	var totalResults int64

	query := r.db.Model(&models.Project{})

	// Apply projectName filter if provided
	if projectName != "" {
		query = query.Where("project_name LIKE ?", "%"+projectName+"%")
	}

	// Apply city filter if provided
	if city != "" {
		query = query.Where("city LIKE ?", "%"+city+"%")
	}

	// Apply date range filter if both dates are provided
	if startDate != "" && endDate != "" {
		// Parse the end date and add one day to include the entire end date
		endDateParsed, err := time.Parse("2006-01-02", endDate)
		if err == nil {
			endDatePlusOne := endDateParsed.Add(24 * time.Hour)
			query = query.Where("created_at >= ? AND created_at <= ?", startDate, endDatePlusOne.Format("2006-01-02"))
		}
	}

	// Get total count before pagination
	if err := query.Count(&totalResults).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&projects).Error
	if err != nil {
		return nil, 0, err
	}

	return projects, totalResults, nil
}

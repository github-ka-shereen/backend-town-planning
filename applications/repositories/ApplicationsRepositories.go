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
	CreateApprovalGroup(tx *gorm.DB, group *models.ApprovalGroup) (*models.ApprovalGroup, error)
	GetApprovalGroupWithMembers(db *gorm.DB, groupID string) (*models.ApprovalGroup, error)
	GetApprovalGroups(db *gorm.DB) ([]models.ApprovalGroup, error)
	GetApprovalGroupByID(db *gorm.DB, groupID string) (*models.ApprovalGroup, error)
	GetFilteredApprovalGroups(limit, offset int, filters map[string]string) ([]models.ApprovalGroup, int64, error)
}

type applicationRepository struct {
	db *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) ApplicationRepository {
	return &applicationRepository{db: db}
}

// CreateApprovalGroup creates a new approval group with its members
func (r *applicationRepository) CreateApprovalGroup(tx *gorm.DB, group *models.ApprovalGroup) (*models.ApprovalGroup, error) {
	// Use transaction to ensure atomicity
	if err := tx.Create(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

// GetApprovalGroupWithMembers fetches an approval group with all its active members and their user details
func (r *applicationRepository) GetApprovalGroupWithMembers(db *gorm.DB, groupID string) (*models.ApprovalGroup, error) {
	var group models.ApprovalGroup

	err := db.
		Preload("Members", "is_active = ?", true).
		Preload("Members.User").
		Preload("Members.User.Department").
		Preload("Members.User.Role").
		Where("id = ?", groupID).
		First(&group).Error

	if err != nil {
		return nil, err
	}

	return &group, nil
}

// GetApprovalGroups fetches all approval groups with basic member info
func (r *applicationRepository) GetApprovalGroups(db *gorm.DB) ([]models.ApprovalGroup, error) {
	var groups []models.ApprovalGroup

	err := db.
		Preload("Members", "is_active = ?", true).
		Preload("Members.User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email")
		}).
		Where("is_active = ?", true).
		Order("created_at DESC").
		Find(&groups).Error

	if err != nil {
		return nil, err
	}

	return groups, nil
}

// GetApprovalGroupByID fetches a single approval group by ID without members
func (r *applicationRepository) GetApprovalGroupByID(db *gorm.DB, groupID string) (*models.ApprovalGroup, error) {
	var group models.ApprovalGroup

	err := db.
		Where("id = ?", groupID).
		First(&group).Error

	if err != nil {
		return nil, err
	}

	return &group, nil
}

func (r *applicationRepository) GetApplicationById(applicationID string) (*models.Application, error) {
	var application models.Application
	if err := r.db.
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Preload("Documents").
		Preload("Payment").
		Where("id = ?", applicationID).
		First(&application).Error; err != nil {
		return nil, err
	}
	return &application, nil
}

// GetFilteredApprovalGroups fetches approval groups with filtering and pagination
func (r *applicationRepository) GetFilteredApprovalGroups(limit, offset int, filters map[string]string) ([]models.ApprovalGroup, int64, error) {
	var approvalGroups []models.ApprovalGroup
	var total int64

	// Start building the query with preloads for members and their users
	query := r.db.Model(&models.ApprovalGroup{}).
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("review_order ASC")
		}).
		Preload("Members.User").
		Preload("Assignments", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true)
		})
		

	// Apply filters
	if name, exists := filters["name"]; exists && name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}

	if groupType, exists := filters["type"]; exists && groupType != "" {
		query = query.Where("type = ?", groupType)
	}

	if isActive, exists := filters["is_active"]; exists && isActive != "" {
		if isActive == "true" {
			query = query.Where("is_active = ?", true)
		} else if isActive == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	if requiresAllApprovals, exists := filters["requires_all_approvals"]; exists && requiresAllApprovals != "" {
		if requiresAllApprovals == "true" {
			query = query.Where("requires_all_approvals = ?", true)
		} else if requiresAllApprovals == "false" {
			query = query.Where("requires_all_approvals = ?", false)
		}
	}

	if autoAssignBackups, exists := filters["auto_assign_backups"]; exists && autoAssignBackups != "" {
		if autoAssignBackups == "true" {
			query = query.Where("auto_assign_backups = ?", true)
		} else if autoAssignBackups == "false" {
			query = query.Where("auto_assign_backups = ?", false)
		}
	}

	if createdBy, exists := filters["created_by"]; exists && createdBy != "" {
		query = query.Where("created_by ILIKE ?", "%"+createdBy+"%")
	}

	// Filter for groups with active members
	if hasActiveMembers, exists := filters["has_active_members"]; exists && hasActiveMembers != "" {
		if hasActiveMembers == "true" {
			query = query.Joins("JOIN approval_group_members ON approval_group_members.approval_group_id = approval_groups.id").
				Where("approval_group_members.is_active = ?", true)
		} else if hasActiveMembers == "false" {
			query = query.Where("NOT EXISTS (?)", 
				r.db.Model(&models.ApprovalGroupMember{}).
					Where("approval_group_members.approval_group_id = approval_groups.id").
					Where("approval_group_members.is_active = ?", true),
			)
		}
	}

	// NEW: Filter for groups that have a final approver
	if hasFinalApprover, exists := filters["has_final_approver"]; exists && hasFinalApprover != "" {
		if hasFinalApprover == "true" {
			query = query.Joins("JOIN approval_group_members ON approval_group_members.approval_group_id = approval_groups.id").
				Where("approval_group_members.is_final_approver = ?", true).
				Where("approval_group_members.is_active = ?", true)
		} else if hasFinalApprover == "false" {
			query = query.Where("NOT EXISTS (?)", 
				r.db.Model(&models.ApprovalGroupMember{}).
					Where("approval_group_members.approval_group_id = approval_groups.id").
					Where("approval_group_members.is_final_approver = ?", true).
					Where("approval_group_members.is_active = ?", true),
			)
		}
	}

	// Count total number of records matching the filters
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated approval groups, ordered by name
	if err := query.
		Order("name ASC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&approvalGroups).Error; err != nil {
		return nil, 0, err
	}

	return approvalGroups, total, nil
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

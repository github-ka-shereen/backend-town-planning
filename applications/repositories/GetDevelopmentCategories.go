package repositories

import (
	"fmt"
	"strings"
	"town-planning-backend/db/models"
)

// GetFilteredDevelopmentCategories fetches development categories with filtering and pagination
func (r *applicationRepository) GetFilteredDevelopmentCategories(pageSize, offset int, filters map[string]string) ([]models.DevelopmentCategory, int64, error) {
	var categories []models.DevelopmentCategory
	var total int64

	query := r.db.Model(&models.DevelopmentCategory{})

	// Apply filters
	if isActive, exists := filters["is_active"]; exists && isActive != "" {
		query = query.Where("is_active = ?", isActive == "true")
	}

	if isSystem, exists := filters["is_system"]; exists && isSystem != "" {
		query = query.Where("is_system = ?", isSystem == "true")
	}

	if search, exists := filters["search"]; exists && search != "" {
		searchPattern := fmt.Sprintf("%%%s%%", strings.ToLower(search))
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", searchPattern, searchPattern)
	}

	if createdBy, exists := filters["created_by"]; exists && createdBy != "" {
		query = query.Where("created_by = ?", createdBy)
	}

	// Count total records before pagination
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and order
	err := query.
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&categories).Error

	if err != nil {
		return nil, 0, err
	}

	return categories, total, nil
}

// Optional: Get all development categories without pagination (for dropdowns, etc.)
func (r *applicationRepository) GetAllDevelopmentCategories(isActive *bool) ([]models.DevelopmentCategory, error) {
	var categories []models.DevelopmentCategory

	query := r.db.Model(&models.DevelopmentCategory{})

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	err := query.
		Order("name ASC").
		Find(&categories).Error

	return categories, err
}

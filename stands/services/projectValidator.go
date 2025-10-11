package services

import (
	"town-planning-backend/db/models"
	"errors"
	"fmt"
	"strings"
	"github.com/google/uuid"
)

// ValidateProject validates a single project instance
func ValidateProject(project *models.Project) string {
	var validationErrors []string

	if project.ProjectNumber == "" {
		validationErrors = append(validationErrors, "Project Number")
	}
	if project.ProjectName == "" {
		validationErrors = append(validationErrors, "Project Name")
	}
	if project.Address == "" {
		validationErrors = append(validationErrors, "Address")
	}
	if project.City == "" {
		validationErrors = append(validationErrors, "City")
	}

	if len(validationErrors) > 0 {
		return fmt.Sprintf("Missing required fields: %s", strings.Join(validationErrors, ", "))
	}
	return ""
}

// ValidateProjectRow validates a row from the Excel file and creates a Project instance
func ValidateProjectRow(row []string, rowIndex int, createdBy string) (models.Project, error) {
	// Trim whitespace from all fields
	projectNumber := strings.TrimSpace(row[0])
	projectName := strings.TrimSpace(row[1])
	address := strings.TrimSpace(row[2])
	city := strings.TrimSpace(row[3])

	// Collect all validation errors
	var validationErrors []string

	if projectNumber == "" {
		validationErrors = append(validationErrors, "Project Number")
	}
	if projectName == "" {
		validationErrors = append(validationErrors, "Project Name")
	}
	if address == "" {
		validationErrors = append(validationErrors, "Address")
	}
	if city == "" {
		validationErrors = append(validationErrors, "City")
	}

	// If there are any validation errors, return a comprehensive error message
	if len(validationErrors) > 0 {
		errorMsg := fmt.Sprintf("Row %d is missing required fields: %s", 
			rowIndex+1, 
			strings.Join(validationErrors, ", "))
		return models.Project{}, errors.New(errorMsg)
	}

	// Create and return a valid project object
	project := models.Project{
		ID:            uuid.New(),
		ProjectNumber: projectNumber,
		ProjectName:   projectName,
		Address:       address,
		City:          city,
		CreatedBy:     createdBy,
	}

	return project, nil
}

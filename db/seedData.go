package db

import (
	"time"
	"town-planning-backend/db/models"

	"gorm.io/gorm"
)

// SeedStandTypes populates the database with system stand types
func SeedStandTypes(db *gorm.DB, createdBy string) error {
	standTypes := []models.StandType{
		{Name: "RESIDENTIAL", Description: "Residential stands", IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "COMMERCIAL", Description: "Commercial stands", IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "INDUSTRIAL", Description: "Industrial stands", IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "AGRICULTURAL", Description: "Agricultural stands", IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "RECREATIONAL", Description: "Recreational stands", IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "INSTITUTIONAL", Description: "Institutional stands", IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "MIXED_USE", Description: "Mixed-use stands", IsSystem: true, IsActive: true, CreatedBy: createdBy},
	}

	for _, st := range standTypes {
		var existing models.StandType
		if err := db.Where("name = ?", st.Name).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&st).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	return nil
}

// SeedDocumentCategories populates the database with system document categories
func SeedDocumentCategories(db *gorm.DB, createdBy string) error {
	categories := []models.DocumentCategory{
		// Legal and Identity Documents
		{
			Name:        "TITLE_DEED",
			Description: "Legal property title deed document",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "ID_COPY",
			Description: "Identification document copy",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "ORGANISATION_REGISTRATION",
			Description: "Organization registration documents",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "POWER_OF_ATTORNEY",
			Description: "Power of attorney documentation",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},

		// Planning Documents
		{
			Name:        "BUILDING_PLANS",
			Description: "Building plans and blueprints",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "SURVEY_PLAN",
			Description: "Survey plan and land measurements",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "SITE_LAYOUT",
			Description: "Site layout and planning documents",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "ARCHITECTURAL_DRAWINGS",
			Description: "Architectural drawings and designs",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "STRUCTURAL_DRAWINGS",
			Description: "Structural engineering drawings",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},

		// Financial Documents
		{
			Name:        "PAYMENT_RECEIPT",
			Description: "Payment receipts and transaction records",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "RATES_CLEARANCE",
			Description: "Rates clearance certificates",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "AGREEMENT_OF_SALE",
			Description: "Agreement of sale contracts",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},

		// Technical Certificates
		{
			Name:        "ENGINEERING_CERTIFICATE",
			Description: "Engineering certificates and approvals",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "LIMPIM_CERTIFICATE",
			Description: "LIMPIM certificates and compliance documents",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "ENVIRONMENTAL_CLEARANCE",
			Description: "Environmental clearance certificates",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},

		// Application Forms
		{
			Name:        "TPD_FORM",
			Description: "TPD application forms",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "APPLICATION_FORM",
			Description: "General application forms",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},

		// Communication
		{
			Name:        "CORRESPONDENCE",
			Description: "Official correspondence and letters",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "NOTIFICATION",
			Description: "Notifications and official communications",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},

		// Other
		{
			Name:        "OTHER",
			Description: "Other miscellaneous documents",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
	}

	for _, category := range categories {
		// Check if category already exists
		var existingCategory models.DocumentCategory
		if err := db.Where("name = ?", category.Name).First(&existingCategory).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Category doesn't exist, create it
				if err := db.Create(&category).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			// Category exists, update it if needed (only non-system fields)
			if existingCategory.IsSystem {
				// For system categories, only update description and active status
				if err := db.Model(&existingCategory).Updates(map[string]interface{}{
					"description": category.Description,
					"is_active":   category.IsActive,
					"updated_at":  time.Now(),
				}).Error; err != nil {
					return err
				}
			} else {
				// For custom categories, update all fields except IsSystem
				if err := db.Model(&existingCategory).Updates(map[string]interface{}{
					"description": category.Description,
					"is_active":   category.IsActive,
					"updated_at":  time.Now(),
				}).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// SeedPropertyTypes populates the database with system property types
func SeedPropertyTypes(db *gorm.DB, createdBy string) error {
	propertyTypes := []models.PropertyType{
		{
			Name:        "INDUSTRIAL",
			Description: "Industrial properties and facilities",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "COMMERCIAL",
			Description: "Commercial properties and business establishments",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "CHURCH",
			Description: "Religious and church properties",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "HIGH_DENSITY_RESIDENTIAL",
			Description: "High density residential properties (apartments, flats)",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "MEDIUM_DENSITY_RESIDENTIAL",
			Description: "Medium density residential properties (townhouses, duplexes)",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "LOW_DENSITY_RESIDENTIAL",
			Description: "Low density residential properties (single family homes)",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "HOLIDAY_HOME",
			Description: "Holiday and vacation homes",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "GOVERNMENT",
			Description: "Government properties and facilities",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "EDUCATIONAL",
			Description: "Educational institutions and facilities",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "HEALTHCARE",
			Description: "Healthcare facilities and medical centers",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
		{
			Name:        "RECREATIONAL",
			Description: "Recreational facilities and entertainment venues",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   createdBy,
		},
	}

	for _, propertyType := range propertyTypes {
		// Check if property type already exists
		var existingPropertyType models.PropertyType
		if err := db.Where("name = ?", propertyType.Name).First(&existingPropertyType).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Property type doesn't exist, create it
				if err := db.Create(&propertyType).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			// Property type exists, update it if needed
			if existingPropertyType.IsSystem {
				// For system types, only update description and active status
				if err := db.Model(&existingPropertyType).Updates(map[string]interface{}{
					"description": propertyType.Description,
					"is_active":   propertyType.IsActive,
					"updated_at":  time.Now(),
				}).Error; err != nil {
					return err
				}
			} else {
				// For custom types, update all fields except IsSystem
				if err := db.Model(&existingPropertyType).Updates(map[string]interface{}{
					"description": propertyType.Description,
					"is_active":   propertyType.IsActive,
					"updated_at":  time.Now(),
				}).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}

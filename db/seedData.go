package db

import (
	"time"
	"town-planning-backend/db/models"

	"gorm.io/gorm"
)

// SeedStandTypes populates the database with system stand types
func SeedStandTypes(db *gorm.DB, createdBy string) error {
	descResidential := "Residential stands"
	descCommercial := "Commercial stands"
	descIndustrial := "Industrial stands"
	descAgricultural := "Agricultural stands"
	descRecreational := "Recreational stands"
	descInstitutional := "Institutional stands"
	descMixedUse := "Mixed-use stands"

	standTypes := []models.StandType{
		{Name: "RESIDENTIAL", Description: &descResidential, IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "COMMERCIAL", Description: &descCommercial, IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "INDUSTRIAL", Description: &descIndustrial, IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "AGRICULTURAL", Description: &descAgricultural, IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "RECREATIONAL", Description: &descRecreational, IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "INSTITUTIONAL", Description: &descInstitutional, IsSystem: true, IsActive: true, CreatedBy: createdBy},
		{Name: "MIXED_USE", Description: &descMixedUse, IsSystem: true, IsActive: true, CreatedBy: createdBy},
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

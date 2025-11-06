package seeds

import (
	"errors"
	"fmt"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedTownPlanningDepartments seeds departments specific to town planning
func SeedTownPlanningDepartments(db *gorm.DB) error {
	config.Logger.Info("Starting town planning departments seeding...")

	departments := []models.Department{
		{
			ID:             uuid.New(),
			Name:           "Town Planning Department",
			Description:    stringPtr("Central department responsible for development control and urban planning"),
			IsActive:       true,
			IsSystem:       true,
			Email:          stringPtr("townplanning@citycouncil.gov.zw"),
			PhoneNumber:    stringPtr("+263-242-123456"),
			OfficeLocation: stringPtr("City Hall, 3rd Floor, Town Planning Wing"),
			CreatedBy:      "system",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			Name:           "Development Control",
			Description:    stringPtr("Section responsible for development applications and approvals"),
			IsActive:       true,
			IsSystem:       true,
			Email:          stringPtr("devcontrol@citycouncil.gov.zw"),
			PhoneNumber:    stringPtr("+263-242-123457"),
			OfficeLocation: stringPtr("City Hall, 3rd Floor, Development Control Section"),
			CreatedBy:      "system",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			Name:           "Building Inspections",
			Description:    stringPtr("Section responsible for construction monitoring and compliance"),
			IsActive:       true,
			IsSystem:       true,
			Email:          stringPtr("buildinginspections@citycouncil.gov.zw"),
			PhoneNumber:    stringPtr("+263-242-123458"),
			OfficeLocation: stringPtr("City Hall, 2nd Floor, Building Inspections Unit"),
			CreatedBy:      "system",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	createdCount := 0
	updatedCount := 0

	for _, department := range departments {
		var existingDepartment models.Department
		result := db.Where("name = ?", department.Name).First(&existingDepartment)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := db.Create(&department).Error; err != nil {
					config.Logger.Error("Failed to create town planning department",
						zap.String("name", department.Name),
						zap.Error(err))
					return fmt.Errorf("failed to create department %s: %w", department.Name, err)
				}
				createdCount++
				config.Logger.Info("Created town planning department", zap.String("name", department.Name))
			} else {
				config.Logger.Error("Error checking for existing town planning department",
					zap.String("name", department.Name),
					zap.Error(result.Error))
				return fmt.Errorf("error checking for department %s: %w", department.Name, result.Error)
			}
		} else {
			// Update existing department
			department.ID = existingDepartment.ID
			if err := db.Model(&existingDepartment).Updates(department).Error; err != nil {
				config.Logger.Error("Failed to update town planning department",
					zap.String("name", department.Name),
					zap.Error(err))
				return fmt.Errorf("failed to update department %s: %w", department.Name, err)
			}
			updatedCount++
			config.Logger.Info("Updated town planning department", zap.String("name", department.Name))
		}
	}

	config.Logger.Info("Town planning departments seeding completed",
		zap.Int("created", createdCount),
		zap.Int("updated", updatedCount))

	return nil
}

// SeedTownPlanningRoles seeds the roles specific to town planning department
func SeedTownPlanningRoles(db *gorm.DB) error {
	config.Logger.Info("Starting town planning roles seeding...")

	roles := []models.Role{
		{
			ID:          uuid.New(),
			Name:        "Town Planning Director",
			Description: "Director of Town Planning Department with full administrative access",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "Town Planning Officer",
			Description: "Senior town planning officer with application review authority",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "Planning Technician",
			Description: "Technical staff for processing applications and documents",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "Building Inspector",
			Description: "Field inspector for construction sites and compliance",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "Public User",
			Description: "External users submitting development applications",
			IsSystem:    true,
			IsActive:    true,
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	createdCount := 0
	updatedCount := 0

	for _, role := range roles {
		var existingRole models.Role
		result := db.Where("name = ?", role.Name).First(&existingRole)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := db.Create(&role).Error; err != nil {
					config.Logger.Error("Failed to create town planning role",
						zap.String("name", role.Name),
						zap.Error(err))
					return fmt.Errorf("failed to create role %s: %w", role.Name, err)
				}
				createdCount++
				config.Logger.Info("Created town planning role", zap.String("name", role.Name))
			} else {
				config.Logger.Error("Error checking for existing town planning role",
					zap.String("name", role.Name),
					zap.Error(result.Error))
				return fmt.Errorf("error checking for role %s: %w", role.Name, result.Error)
			}
		} else {
			// Update existing role
			role.ID = existingRole.ID
			if err := db.Model(&existingRole).Updates(role).Error; err != nil {
				config.Logger.Error("Failed to update town planning role",
					zap.String("name", role.Name),
					zap.Error(err))
				return fmt.Errorf("failed to update role %s: %w", role.Name, err)
			}
			updatedCount++
			config.Logger.Info("Updated town planning role", zap.String("name", role.Name))
		}
	}

	config.Logger.Info("Town planning roles seeding completed",
		zap.Int("created", createdCount),
		zap.Int("updated", updatedCount))

	return nil
}

// SeedTownPlanningPermissions seeds permissions specific to town planning operations
func SeedTownPlanningPermissions(db *gorm.DB) error {
	config.Logger.Info("Starting town planning permissions seeding...")

	permissions := []models.Permission{
		// Application Management
		{ID: uuid.New(), Name: "application.submit", Description: "Submit new development applications", Resource: "applications", Action: "create", Category: "application_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "application.read", Description: "View development applications", Resource: "applications", Action: "read", Category: "application_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "application.update", Description: "Update development applications", Resource: "applications", Action: "update", Category: "application_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "application.review", Description: "Review and assess development applications", Resource: "applications", Action: "update", Category: "application_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "application.approve", Description: "Approve development applications", Resource: "applications", Action: "update", Category: "application_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "application.reject", Description: "Reject development applications", Resource: "applications", Action: "update", Category: "application_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// Document Management
		{ID: uuid.New(), Name: "document.upload", Description: "Upload application documents", Resource: "documents", Action: "create", Category: "document_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "document.read", Description: "View application documents", Resource: "documents", Action: "read", Category: "document_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "document.process", Description: "Process application documents", Resource: "documents", Action: "update", Category: "document_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "document.generate.tpd1", Description: "Generate TPD-1 forms", Resource: "documents", Action: "create", Category: "document_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// Payment Processing
		{ID: uuid.New(), Name: "payment.process", Description: "Process application payments", Resource: "payments", Action: "create", Category: "financial_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "payment.verify", Description: "Verify payment receipts", Resource: "payments", Action: "read", Category: "financial_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// Inspection Management
		{ID: uuid.New(), Name: "inspection.schedule", Description: "Schedule site inspections", Resource: "inspections", Action: "create", Category: "inspection_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "inspection.conduct", Description: "Conduct site inspections", Resource: "inspections", Action: "create", Category: "inspection_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// User Management (for directors/officers)
		{ID: uuid.New(), Name: "user.manage", Description: "Manage system users", Resource: "users", Action: "create", Category: "user_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "user.read", Description: "View user information", Resource: "users", Action: "read", Category: "user_management", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// Reporting
		{ID: uuid.New(), Name: "report.generate", Description: "Generate system reports", Resource: "reports", Action: "read", Category: "reporting", IsActive: true, CreatedBy: "system", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	createdCount := 0
	updatedCount := 0

	for _, permission := range permissions {
		var existingPermission models.Permission
		result := db.Where("name = ?", permission.Name).First(&existingPermission)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := db.Create(&permission).Error; err != nil {
					config.Logger.Error("Failed to create permission",
						zap.String("name", permission.Name),
						zap.Error(err))
					return fmt.Errorf("failed to create permission %s: %w", permission.Name, err)
				}
				createdCount++
				config.Logger.Info("Created permission", zap.String("name", permission.Name))
			} else {
				config.Logger.Error("Error checking for existing permission",
					zap.String("name", permission.Name),
					zap.Error(result.Error))
				return fmt.Errorf("error checking for permission %s: %w", permission.Name, result.Error)
			}
		} else {
			// Update existing permission
			permission.ID = existingPermission.ID
			if err := db.Model(&existingPermission).Updates(permission).Error; err != nil {
				config.Logger.Error("Failed to update permission",
					zap.String("name", permission.Name),
					zap.Error(err))
				return fmt.Errorf("failed to update permission %s: %w", permission.Name, err)
			}
			updatedCount++
			config.Logger.Info("Updated permission", zap.String("name", permission.Name))
		}
	}

	config.Logger.Info("Permissions seeding completed",
		zap.Int("created", createdCount),
		zap.Int("updated", updatedCount))

	return nil
}

// SeedTownPlanningDocumentCategories seeds document categories for town planning
func SeedTownPlanningDocumentCategories(db *gorm.DB) error {
	config.Logger.Info("Starting town planning document categories seeding...")
	createdBy := "system"

	categories := []models.DocumentCategory{
		// Application Process Documents
		{ID: uuid.New(), Name: "Development Application Form", Code: "DEVELOPMENT_APPLICATION", Description: "Main development application forms", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "TPD-1 Form", Code: "TPD1_FORM", Description: "Official TPD-1 application forms", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Processed Receipt", Code: "PROCESSED_RECEIPT", Description: "Payment receipts for development applications", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Processed Quotation", Code: "PROCESSED_QUOTATION", Description: "Quotations for development services", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Development Permit Quotation", Code: "DEVELOPMENT_PERMIT_QUOTATION", Description: "Quotations for development permit applications", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Chat Attachments", Code: "CHAT_ATTACHMENT", Description: "Files attached to chat messages and discussions", IsSystem: true, CreatedBy: createdBy},

		// Site and Building Plans
		{ID: uuid.New(), Name: "Initial Building Plan", Code: "INITIAL_PLAN", Description: "Initial architectural building plans", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Site Plan", Code: "SITE_PLAN", Description: "Property site and layout plans", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Building Plan", Code: "BUILDING_PLAN", Description: "Detailed architectural building plans", IsSystem: true, CreatedBy: createdBy},

		// Engineering and Structural Documents
		{ID: uuid.New(), Name: "Structural Engineering Certificate", Code: "ENGINEERING_CERTIFICATE", Description: "Structural engineering certificates", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Ring Beam Certificate", Code: "RING_BEAM_CERTIFICATE", Description: "Ring beam construction certificates", IsSystem: true, CreatedBy: createdBy},

		// Legal and Ownership Documents
		{ID: uuid.New(), Name: "Title Deed", Code: "TITLE_DEED", Description: "Property title deeds", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Survey Diagram", Code: "SURVEY_DIAGRAM", Description: "Land survey diagrams", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Lease Agreement", Code: "LEASE_AGREEMENT", Description: "Property lease agreements", IsSystem: true, CreatedBy: createdBy},

		// Identity and Personal Documents
		{ID: uuid.New(), Name: "National ID", Code: "NATIONAL_ID", Description: "National identification documents", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Proof of Residence", Code: "PROOF_OF_RESIDENCE", Description: "Proof of residence documents", IsSystem: true, CreatedBy: createdBy},

		// Technical Reports
		{ID: uuid.New(), Name: "Geotechnical Report", Code: "GEOTECHNICAL_REPORT", Description: "Soil and geotechnical reports", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Environmental Impact Assessment", Code: "EIA_REPORT", Description: "Environmental impact assessments", IsSystem: true, CreatedBy: createdBy},

		// Approval and Permit Documents
		{ID: uuid.New(), Name: "Development Permit", Code: "DEVELOPMENT_PERMIT", Description: "Development permits and approvals", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Building Permit", Code: "BUILDING_PERMIT", Description: "Building construction permits", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Occupation Certificate", Code: "OCCUPATION_CERTIFICATE", Description: "Certificate of occupation", IsSystem: true, CreatedBy: createdBy},

		// Inspection Documents
		{ID: uuid.New(), Name: "Site Inspection Report", Code: "INSPECTION_REPORT", Description: "Site inspection reports", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Compliance Certificate", Code: "COMPLIANCE_CERTIFICATE", Description: "Building compliance certificates", IsSystem: true, CreatedBy: createdBy},

		// Correspondence
		{ID: uuid.New(), Name: "Official Correspondence", Code: "OFFICIAL_CORRESPONDENCE", Description: "Official letters and correspondence", IsSystem: true, CreatedBy: createdBy},
		{ID: uuid.New(), Name: "Approval Letter", Code: "APPROVAL_LETTER", Description: "Approval and decision letters", IsSystem: true, CreatedBy: createdBy},

		{ID: uuid.New(), Name: "Other Documents", Code: "OTHER", Description: "Other uncategorized documents", IsSystem: true, CreatedBy: createdBy},
	}

	createdCount := 0
	updatedCount := 0

	for _, category := range categories {
		// Set timestamps
		category.CreatedAt = time.Now()
		category.UpdatedAt = time.Now()
		category.IsActive = true

		// Check if category already exists
		var existingCategory models.DocumentCategory
		result := db.Where("code = ?", category.Code).First(&existingCategory)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// Create new category
				if err := db.Create(&category).Error; err != nil {
					config.Logger.Error("Failed to create document category",
						zap.String("code", category.Code),
						zap.Error(err))
					return fmt.Errorf("failed to create document category %s: %w", category.Code, err)
				}
				createdCount++
				config.Logger.Info("Created document category",
					zap.String("name", category.Name),
					zap.String("code", category.Code))
			} else {
				config.Logger.Error("Error checking for existing document category",
					zap.String("code", category.Code),
					zap.Error(result.Error))
				return fmt.Errorf("error checking for document category %s: %w", category.Code, result.Error)
			}
		} else {
			// Update existing category
			category.ID = existingCategory.ID
			if err := db.Model(&existingCategory).Updates(category).Error; err != nil {
				config.Logger.Error("Failed to update document category",
					zap.String("code", category.Code),
					zap.Error(err))
				return fmt.Errorf("failed to update document category %s: %w", category.Code, err)
			}
			updatedCount++
			config.Logger.Info("Updated document category",
				zap.String("name", category.Name),
				zap.String("code", category.Code))
		}
	}

	config.Logger.Info("Document categories seeding completed",
		zap.Int("created", createdCount),
		zap.Int("updated", updatedCount))

	return nil
}

// SeedTownPlanningUsers seeds initial users for town planning department
func SeedTownPlanningUsers(db *gorm.DB) error {
	config.Logger.Info("Starting town planning users seeding...")

	// Get roles
	var directorRole models.Role
	if err := db.Where("name = ?", "Town Planning Director").First(&directorRole).Error; err != nil {
		return fmt.Errorf("town planning director role not found: %w", err)
	}

	var officerRole models.Role
	if err := db.Where("name = ?", "Town Planning Officer").First(&officerRole).Error; err != nil {
		return fmt.Errorf("town planning officer role not found: %w", err)
	}

	var technicianRole models.Role
	if err := db.Where("name = ?", "Planning Technician").First(&technicianRole).Error; err != nil {
		return fmt.Errorf("planning technician role not found: %w", err)
	}

	// Get department
	var planningDept models.Department
	if err := db.Where("name = ?", "Town Planning Department").First(&planningDept).Error; err != nil {
		return fmt.Errorf("town planning department not found: %w", err)
	}

	users := []models.User{
		{
			ID:            uuid.New(),
			FirstName:     "Director",
			LastName:      "Town Planning",
			Email:         "director.townplanning@citycouncil.gov.zw",
			Phone:         "+263242111111",
			AuthMethod:    models.AuthMethodMagicLink,
			RoleID:        directorRole.ID,
			DepartmentID:  &planningDept.ID,
			Active:        true,
			IsSuspended:   false,
			EmailVerified: true,
			CreatedBy:     "system",
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		},
		{
			ID:            uuid.New(),
			FirstName:     "Senior",
			LastName:      "Planning Officer",
			Email:         "officer.planning@citycouncil.gov.zw",
			Phone:         "+263242111112",
			AuthMethod:    models.AuthMethodMagicLink,
			RoleID:        officerRole.ID,
			DepartmentID:  &planningDept.ID,
			Active:        true,
			IsSuspended:   false,
			EmailVerified: true,
			CreatedBy:     "system",
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		},
		{
			ID:            uuid.New(),
			FirstName:     "Planning",
			LastName:      "Technician",
			Email:         "technician.planning@citycouncil.gov.zw",
			Phone:         "+263242111113",
			AuthMethod:    models.AuthMethodMagicLink,
			RoleID:        technicianRole.ID,
			DepartmentID:  &planningDept.ID,
			Active:        true,
			IsSuspended:   false,
			EmailVerified: true,
			CreatedBy:     "system",
			CreatedAt:     time.Now(),
			LastUpdatedAt: time.Now(),
		},
	}

	createdCount := 0
	for _, user := range users {
		var existingUser models.User
		result := db.Where("email = ?", user.Email).First(&existingUser)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// Hash password
				hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
				if err != nil {
					return fmt.Errorf("failed to hash password for %s: %w", user.Email, err)
				}
				user.Password = string(hashedPassword)

				if err := db.Create(&user).Error; err != nil {
					config.Logger.Error("Failed to create user",
						zap.String("email", user.Email),
						zap.Error(err))
					return fmt.Errorf("failed to create user %s: %w", user.Email, err)
				}
				createdCount++
				config.Logger.Info("Created town planning user",
					zap.String("name", user.FirstName+" "+user.LastName),
					zap.String("email", user.Email),
					zap.String("role", user.RoleID.String()))
			} else {
				return fmt.Errorf("error checking for user %s: %w", user.Email, result.Error)
			}
		}
	}

	config.Logger.Info("Town planning users seeding completed", zap.Int("created", createdCount))
	return nil
}

// CreateTownPlanningRolePermissions assigns permissions to town planning roles
func CreateTownPlanningRolePermissions(db *gorm.DB) error {
	config.Logger.Info("Starting town planning role permission assignments...")

	// Get all roles
	roles := map[string]models.Role{}
	roleNames := []string{"Town Planning Director", "Town Planning Officer", "Planning Technician", "Building Inspector", "Public User"}

	for _, roleName := range roleNames {
		var role models.Role
		if err := db.Where("name = ?", roleName).First(&role).Error; err != nil {
			config.Logger.Warn("Role not found, skipping", zap.String("role", roleName))
			continue
		}
		roles[roleName] = role
	}

	// Define role permissions
	rolePermissions := map[string][]string{
		"Town Planning Director": {
			// Full access
			"application.submit", "application.read", "application.update", "application.review", "application.approve", "application.reject",
			"document.upload", "document.read", "document.process", "document.generate.tpd1",
			"payment.process", "payment.verify",
			"inspection.schedule", "inspection.conduct",
			"user.manage", "user.read",
			"report.generate",
		},
		"Town Planning Officer": {
			// Application review and approval
			"application.read", "application.review", "application.approve", "application.reject",
			"document.read", "document.process",
			"payment.verify",
			"inspection.schedule", "inspection.conduct",
			"user.read",
			"report.generate",
		},
		"Planning Technician": {
			// Document processing and basic application handling
			"application.submit", "application.read", "application.update",
			"document.upload", "document.read", "document.process", "document.generate.tpd1",
			"payment.process", "payment.verify",
			"user.read",
		},
		"Building Inspector": {
			// Inspection focused
			"application.read",
			"document.read",
			"inspection.schedule", "inspection.conduct",
			"user.read",
		},
		"Public User": {
			// Basic submission and viewing
			"application.submit", "application.read",
			"document.upload", "document.read",
		},
	}

	totalAssignments := 0
	for roleName, permissionNames := range rolePermissions {
		role, exists := roles[roleName]
		if !exists {
			config.Logger.Warn("Role not found for permission assignment", zap.String("role", roleName))
			continue
		}

		roleAssignments := 0
		for _, permName := range permissionNames {
			var permission models.Permission
			if err := db.Where("name = ?", permName).First(&permission).Error; err != nil {
				config.Logger.Warn("Permission not found, skipping", zap.String("permission", permName))
				continue
			}

			// Check if already assigned
			var existingRolePermission models.RolePermission
			err := db.Where("role_id = ? AND permission_id = ?", role.ID, permission.ID).First(&existingRolePermission).Error

			if errors.Is(err, gorm.ErrRecordNotFound) {
				rolePermission := models.RolePermission{
					ID:           uuid.New(),
					RoleID:       role.ID,
					PermissionID: permission.ID,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}

				if err := db.Create(&rolePermission).Error; err != nil {
					config.Logger.Error("Failed to create role permission",
						zap.String("role", role.Name),
						zap.String("permission", permission.Name),
						zap.Error(err))
					return fmt.Errorf("failed to create role permission for %s: %w", permission.Name, err)
				}
				roleAssignments++
				totalAssignments++
			}
		}
		config.Logger.Info("Role permissions assigned",
			zap.String("role", roleName),
			zap.Int("permissions", roleAssignments))
	}

	config.Logger.Info("Town planning role permission assignments completed",
		zap.Int("total_assignments", totalAssignments))

	return nil
}

// SeedTownPlanningAll runs all town planning seeding functions in correct order
func SeedTownPlanningAll(db *gorm.DB) error {
	config.Logger.Info("Starting comprehensive town planning database seeding...")

	// Seed in order of dependencies
	if err := SeedTownPlanningDepartments(db); err != nil {
		return fmt.Errorf("failed to seed departments: %w", err)
	}

	if err := SeedTownPlanningRoles(db); err != nil {
		return fmt.Errorf("failed to seed roles: %w", err)
	}

	if err := SeedTownPlanningPermissions(db); err != nil {
		return fmt.Errorf("failed to seed permissions: %w", err)
	}

	if err := SeedTownPlanningDocumentCategories(db); err != nil {
		return fmt.Errorf("failed to seed document categories: %w", err)
	}

	if err := CreateTownPlanningRolePermissions(db); err != nil {
		return fmt.Errorf("failed to create role permission associations: %w", err)
	}

	if err := SeedTownPlanningUsers(db); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	config.Logger.Info("All town planning database seeding completed successfully")
	return nil
}

func stringPtr(s string) *string {
	return &s
}

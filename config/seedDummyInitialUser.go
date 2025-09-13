package config

import (
	"errors"
	"fmt"
	"log"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedDummyInitialUser creates a single dummy user for initial system access
func SeedDummyInitialUser(db *gorm.DB) error {
	// ======================
	// Create Dummy User (if doesn't exist)
	// ======================
	dummyEmail := "john.doe@example.com"

	// First check if user already exists
	var existingUser models.User
	err := db.Where("email = ?", dummyEmail).First(&existingUser).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// First, we need to ensure we have a role to assign
			var adminRole models.Role
			err = db.Where("code = ?", "admin").First(&adminRole).Error
			if err != nil {
				log.Printf("Admin role not found, please seed roles first: %v", err)
				return fmt.Errorf("admin role not found: %w", err)
			}

			// Optionally find a department (can be nil)
			var department models.Department
			var departmentID *uuid.UUID
			err = db.Where("code = ?", "it").First(&department).Error
			if err == nil {
				departmentID = &department.ID
			}

			// Hash the password properly
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("Failed to hash password: %v", err)
				return fmt.Errorf("failed to hash password: %w", err)
			}

			// User doesn't exist, create new one
			dummyUser := models.User{
				ID:             uuid.New(),
				FirstName:      "John",
				LastName:       "Doe",
				Email:          dummyEmail,
				Password:       string(hashedPassword),
				Phone:          "+263771234567",
				WhatsAppNumber: stringPtr("+263771234567"),
				AuthMethod:     models.AuthMethodMagicLink,
				RoleID:         adminRole.ID,
				DepartmentID:   departmentID,
				Active:         true,
				IsSuspended:    false,
				EmailVerified:  true,
				CreatedBy:      "system",
			}

			// Use direct DB creation with explicit context
			if err := db.Create(&dummyUser).Error; err != nil {
				log.Printf("Failed to create dummy user: %v", err)
				return fmt.Errorf("failed to create dummy user: %w", err)
			}

			log.Printf("Dummy user created: %s %s (%s)", dummyUser.FirstName, dummyUser.LastName, dummyUser.Email)
		} else {
			log.Printf("Error checking for existing user: %v", err)
			return fmt.Errorf("error checking for existing user: %w", err)
		}
	} else {
		log.Printf("Dummy user already exists: %s %s (%s)", existingUser.FirstName, existingUser.LastName, existingUser.Email)
	}

	return nil
}

// SeedDummyData creates comprehensive initial data including roles, departments, and users
func SeedDummyData(db *gorm.DB) error {
	// 1. Create Admin Role if not exists
	var adminRole models.Role
	err := db.Where("code = ?", "admin").First(&adminRole).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		adminRole = models.Role{
			ID:          uuid.New(),
			Name:        "Administrator",
			Description: "System administrator with full access",
			Code:        "admin",
			IsSystem:    true,
			IsActive:    true,
			Level:       100,
			CreatedBy:   "system",
		}

		if err := db.Create(&adminRole).Error; err != nil {
			return fmt.Errorf("failed to create admin role: %w", err)
		}
		log.Printf("Admin role created: %s", adminRole.Name)
	} else if err != nil {
		return fmt.Errorf("error checking for admin role: %w", err)
	}

	// 2. Create User Role if not exists
	var userRole models.Role
	err = db.Where("code = ?", "user").First(&userRole).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		userRole = models.Role{
			ID:          uuid.New(),
			Name:        "User",
			Description: "Standard user with limited access",
			Code:        "user",
			IsSystem:    false,
			IsActive:    true,
			Level:       10,
			CreatedBy:   "system",
		}

		if err := db.Create(&userRole).Error; err != nil {
			return fmt.Errorf("failed to create user role: %w", err)
		}
		log.Printf("User role created: %s", userRole.Name)
	} else if err != nil {
		return fmt.Errorf("error checking for user role: %w", err)
	}

	// 3. Create IT Department if not exists
	var itDepartment models.Department
	err = db.Where("code = ?", "it").First(&itDepartment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		itDepartment = models.Department{
			ID:             uuid.New(),
			Name:           "Information Technology",
			Description:    "IT Department responsible for system administration",
			Code:           "it",
			IsActive:       true,
			Email:          stringPtr("it@example.com"),
			PhoneNumber:    stringPtr("+263771111111"),
			OfficeLocation: stringPtr("Building A, Floor 3"),
			CreatedBy:      "system",
		}

		if err := db.Create(&itDepartment).Error; err != nil {
			return fmt.Errorf("failed to create IT department: %w", err)
		}
		log.Printf("IT Department created: %s", itDepartment.Name)
	} else if err != nil {
		return fmt.Errorf("error checking for IT department: %w", err)
	}

	// 4. Create Planning Department if not exists
	var planningDepartment models.Department
	err = db.Where("code = ?", "planning").First(&planningDepartment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		planningDepartment = models.Department{
			ID:             uuid.New(),
			Name:           "Town Planning",
			Description:    "Town Planning Department responsible for development applications",
			Code:           "planning",
			IsActive:       true,
			Email:          stringPtr("planning@example.com"),
			PhoneNumber:    stringPtr("+263772222222"),
			OfficeLocation: stringPtr("Building B, Floor 1"),
			CreatedBy:      "system",
		}

		if err := db.Create(&planningDepartment).Error; err != nil {
			return fmt.Errorf("failed to create Planning department: %w", err)
		}
		log.Printf("Planning Department created: %s", planningDepartment.Name)
	} else if err != nil {
		return fmt.Errorf("error checking for Planning department: %w", err)
	}

	// 5. Create Admin User if not exists
	adminEmail := "admin@example.com"
	var existingAdminUser models.User
	err = db.Where("email = ?", adminEmail).First(&existingAdminUser).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash admin password: %w", err)
		}

		adminUser := models.User{
			ID:             uuid.New(),
			FirstName:      "System",
			LastName:       "Administrator",
			Email:          adminEmail,
			Password:       string(hashedPassword),
			Phone:          "+263771234567",
			WhatsAppNumber: stringPtr("+263771234567"),
			AuthMethod:     models.AuthMethodMagicLink,
			RoleID:         adminRole.ID,
			DepartmentID:   &itDepartment.ID,
			Active:         true,
			IsSuspended:    false,
			EmailVerified:  true,
			CreatedBy:      "system",
		}

		// Debug: Print the values before insertion
		log.Printf("Creating admin user with RoleID: %s, DepartmentID: %v", adminUser.RoleID, adminUser.DepartmentID)

		if err := db.Create(&adminUser).Error; err != nil {
			log.Printf("Failed to create admin user: %v", err)
			return fmt.Errorf("failed to create admin user: %w", err)
		}

		log.Printf("Admin user created: %s %s (%s)", adminUser.FirstName, adminUser.LastName, adminUser.Email)
	} else if err != nil {
		return fmt.Errorf("error checking for existing admin user: %w", err)
	} else {
		log.Printf("Admin user already exists: %s %s (%s)", existingAdminUser.FirstName, existingAdminUser.LastName, existingAdminUser.Email)
	}

	// 6. Create Planning Officer User if not exists
	plannerEmail := "planner@example.com"
	var existingPlannerUser models.User
	err = db.Where("email = ?", plannerEmail).First(&existingPlannerUser).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("planner123"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash planner password: %w", err)
		}

		plannerUser := models.User{
			ID:             uuid.New(),
			FirstName:      "Jane",
			LastName:       "Planner",
			Email:          plannerEmail,
			Password:       string(hashedPassword),
			Phone:          "+263773334444",
			WhatsAppNumber: stringPtr("+263773334444"),
			AuthMethod:     models.AuthMethodPassword,
			RoleID:         userRole.ID,
			DepartmentID:   &planningDepartment.ID,
			Active:         true,
			IsSuspended:    false,
			EmailVerified:  true,
			CreatedBy:      "system",
		}

		// Debug: Print the values before insertion
		log.Printf("Creating planner user with RoleID: %s, DepartmentID: %v", plannerUser.RoleID, plannerUser.DepartmentID)

		if err := db.Create(&plannerUser).Error; err != nil {
			log.Printf("Failed to create planner user: %v", err)
			return fmt.Errorf("failed to create planner user: %w", err)
		}

		log.Printf("Planner user created: %s %s (%s)", plannerUser.FirstName, plannerUser.LastName, plannerUser.Email)
	} else if err != nil {
		return fmt.Errorf("error checking for existing planner user: %w", err)
	} else {
		log.Printf("Planner user already exists: %s %s (%s)", existingPlannerUser.FirstName, existingPlannerUser.LastName, existingPlannerUser.Email)
	}

	return nil
}

// SeedBasicPermissions creates basic system permissions
func SeedBasicPermissions(db *gorm.DB) error {
	permissions := []models.Permission{
		{
			ID:          uuid.New(),
			Name:        "user.create",
			Description: "Create new users",
			Resource:    "users",
			Action:      "create",
			Category:    "user_management",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			ID:          uuid.New(),
			Name:        "user.read",
			Description: "View user information",
			Resource:    "users",
			Action:      "read",
			Category:    "user_management",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			ID:          uuid.New(),
			Name:        "user.update",
			Description: "Update user information",
			Resource:    "users",
			Action:      "update",
			Category:    "user_management",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			ID:          uuid.New(),
			Name:        "user.delete",
			Description: "Delete users",
			Resource:    "users",
			Action:      "delete",
			Category:    "user_management",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			ID:          uuid.New(),
			Name:        "application.create",
			Description: "Create new applications",
			Resource:    "applications",
			Action:      "create",
			Category:    "application_management",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			ID:          uuid.New(),
			Name:        "application.read",
			Description: "View application information",
			Resource:    "applications",
			Action:      "read",
			Category:    "application_management",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			ID:          uuid.New(),
			Name:        "application.update",
			Description: "Update application information",
			Resource:    "applications",
			Action:      "update",
			Category:    "application_management",
			IsActive:    true,
			CreatedBy:   "system",
		},
		{
			ID:          uuid.New(),
			Name:        "application.delete",
			Description: "Delete applications",
			Resource:    "applications",
			Action:      "delete",
			Category:    "application_management",
			IsActive:    true,
			CreatedBy:   "system",
		},
	}

	for _, permission := range permissions {
		var existingPermission models.Permission
		err := db.Where("name = ?", permission.Name).First(&existingPermission).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&permission).Error; err != nil {
				return fmt.Errorf("failed to create permission %s: %w", permission.Name, err)
			}
			log.Printf("Permission created: %s", permission.Name)
		} else if err != nil {
			return fmt.Errorf("error checking for permission %s: %w", permission.Name, err)
		}
	}

	return nil
}

// CreateRolePermissionAssociations assigns permissions to roles
func CreateRolePermissionAssociations(db *gorm.DB) error {
	// Get admin role
	var adminRole models.Role
	if err := db.Where("code = ?", "admin").First(&adminRole).Error; err != nil {
		return fmt.Errorf("admin role not found: %w", err)
	}

	// Get all permissions
	var permissions []models.Permission
	if err := db.Find(&permissions).Error; err != nil {
		return fmt.Errorf("failed to fetch permissions: %w", err)
	}

	// Assign all permissions to admin role
	for _, permission := range permissions {
		var existingRolePermission models.RolePermission
		err := db.Where("role_id = ? AND permission_id = ?", adminRole.ID, permission.ID).First(&existingRolePermission).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			rolePermission := models.RolePermission{
				ID:           uuid.New(),
				RoleID:       adminRole.ID,
				PermissionID: permission.ID,
			}

			if err := db.Create(&rolePermission).Error; err != nil {
				return fmt.Errorf("failed to create role permission for %s: %w", permission.Name, err)
			}
			log.Printf("Permission %s assigned to admin role", permission.Name)
		}
	}

	// Get user role
	var userRole models.Role
	if err := db.Where("code = ?", "user").First(&userRole).Error; err != nil {
		return fmt.Errorf("user role not found: %w", err)
	}

	// Assign limited permissions to user role (only specific application permissions)
	applicationPermissions := []string{"application.read", "application.create"}
	for _, permName := range applicationPermissions {
		var permission models.Permission
		if err := db.Where("name = ?", permName).First(&permission).Error; err != nil {
			log.Printf("Permission %s not found, skipping", permName)
			continue
		}

		var existingRolePermission models.RolePermission
		err := db.Where("role_id = ? AND permission_id = ?", userRole.ID, permission.ID).First(&existingRolePermission).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			rolePermission := models.RolePermission{
				ID:           uuid.New(),
				RoleID:       userRole.ID,
				PermissionID: permission.ID,
			}

			if err := db.Create(&rolePermission).Error; err != nil {
				return fmt.Errorf("failed to create role permission for %s: %w", permission.Name, err)
			}
			log.Printf("Permission %s assigned to user role", permission.Name)
		}
	}

	return nil
}

// RunAllSeeders runs all seeding functions in proper order
func RunAllSeeders(db *gorm.DB) error {
	log.Println("Starting data seeding...")

	// Seed permissions first
	if err := SeedBasicPermissions(db); err != nil {
		return fmt.Errorf("failed to seed permissions: %w", err)
	}

	// Seed roles, departments, and users
	if err := SeedDummyData(db); err != nil {
		return fmt.Errorf("failed to seed dummy data: %w", err)
	}

	// Create role-permission associations
	if err := CreateRolePermissionAssociations(db); err != nil {
		return fmt.Errorf("failed to create role-permission associations: %w", err)
	}

	log.Println("Data seeding completed successfully!")
	return nil
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

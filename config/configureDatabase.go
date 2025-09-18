package config

import (
	"fmt"
	"log"
	"strings"
	"time"
	"town-planning-backend/db/models"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// allModels defines all models that should be migrated and have permissions generated
// This is the only place you need to add new models
var allModels = []interface{}{
	// Core Authentication and Authorization Models
	&models.Permission{},
	&models.Role{},
	&models.RolePermission{},
	&models.Department{},
	&models.User{},
	&models.UserAuditLog{},

	// Applicants (when uncommented)
	// &models.Applicant{},
	// &models.OrganisationRepresentative{},
	// &models.ApplicantDocument{},
	// &models.ApplicantAdditionalPhoneNumbers{},
	// &models.ApplicantOrganisationRepresentative{},

	// Applications (when uncommented)
	// &models.Application{},
	// &models.ApplicationCategory{},
	// &models.RequiredDocument{},
	// &models.ApplicationReview{},
	// &models.Comment{},

	// Documents (when uncommented)
	// &models.Document{},
}

func ConfigureDatabase() *gorm.DB {
	host := GetEnv("DB_HOST")
	user := GetEnv("POSTGRES_USER")
	password := GetEnv("POSTGRES_PASSWORD")
	dbname := GetEnv("POSTGRES_DB")
	port := GetEnv("DB_PORT")
	timezone := GetEnv("DB_TIMEZONE")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=%s",
		host, user, password, dbname, port, timezone,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("[DB-CONNECT] Failed to connect to database: %v", err)
	}

	// Get list of all tables in the database
	var tables []string
	if err := db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tables).Error; err != nil {
		log.Fatalf("Failed to get table list: %v", err)
	}

	// ===== ADD YOUR CLEANUP CODE HERE =====
	// // Define a skip list for tables that should NOT be dropped
	// skipTables := map[string]bool{
	// 	"users":      true, // Example: keep users table
	// 	"clients":    true, // Example: keep clients table
	// 	"email_logs": true, // Example: keep email_logs table
	// 	// Add other tables to preserve here
	// }

	// // Drop all tables EXCEPT skipTables
	// for _, table := range tables {
	// 	if skipTables[table] {
	// 		log.Printf("[DB-CLEANUP] Skipping table: %s", table)
	// 		continue // Skip tables in the skip list
	// 	}

	// 	if err := db.Migrator().DropTable(table); err != nil {
	// 		log.Printf("Failed to drop table %s: %v", table, err) // Continue instead of fatal
	// 	} else {
	// 		log.Printf("[DB-CLEANUP] Dropped table: %s", table)
	// 	}
	// }

	// If you need to drop specific tables in a particular order (like join tables first),
	// you can add that logic here:
	/*
		// Drop join tables first to avoid foreign key constraints
		if err := db.Migrator().DropTable("stand_swap_owners"); err != nil {
			log.Printf("Failed to drop stand_swap_owners join table: %v", err)
		}

		// Then drop other specific tables
		specificTablesToDrop := []string{
			"communication_recipients",
			"communication_attachments",
			"communication_groups",
			"communications",
		}

		for _, table := range specificTablesToDrop {
			if err := db.Migrator().DropTable(table); err != nil {
				log.Printf("Failed to drop table %s: %v", table, err)
			}
		}
	*/
	// ===== END OF CLEANUP CODE =====

	// Auto-migrate all models using the allModels slice
	err = db.AutoMigrate(allModels...)
	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	} else {
		log.Println("Tables migrated successfully")
	}

	// ===== Auto-generate permissions for all models =====
	// // Define models to skip permission generation for (e.g., join tables, audit logs)
	// skipList := map[string]bool{
	// 	"permissions":                            true, // Changed from "permission"
	// 	"role_permissions":                       true, // Changed from "role_permission"
	// 	"user_audit_logs":                        true, // Changed from "user_audit_log"
	// 	"applicant_organisation_representatives": true, // Join table
	// 	// Add other tables to skip here
	// }

	// log.Println("[PERMISSIONS] Starting automatic CRUD permission generation...")

	// // Generate permissions for each model automatically
	// for _, model := range allModels {
	// 	modelType := reflect.TypeOf(model)
	// 	if modelType.Kind() == reflect.Ptr {
	// 		modelType = modelType.Elem()
	// 	}

	// 	// Get the actual GORM table name
	// 	stmt := &gorm.Statement{DB: db}
	// 	stmt.Parse(model)
	// 	tableName := stmt.Schema.Table

	// 	// DEBUG: Print the actual table name
	// 	log.Printf("[DEBUG] Model: %s, Table: %s", modelType.Name(), tableName)

	// 	// Skip tables that don't need CRUD permissions
	// 	if skipList[tableName] {
	// 		log.Printf("[PERMISSIONS] Skipping permission generation for table: %s", tableName)
	// 		continue
	// 	}

	// 	// Use the model struct name as the resource (e.g., "User" -> "users")
	// 	resource := Pluralize(strings.ToLower(modelType.Name()))
	// 	category := strings.ToLower(modelType.Name()) + "_management"

	// 	if err := GenerateModelPermissions(db, resource, category, "system"); err != nil {
	// 		log.Printf("[PERMISSIONS] Failed to generate permissions for %s: %v", resource, err)
	// 	} else {
	// 		log.Printf("[PERMISSIONS] ✓ Generated CRUD permissions for: %s", resource)
	// 	}
	// }

	// log.Println("[PERMISSIONS] Automatic permission generation completed")

	// ===== END OF PERMISSIONS GENERATION =====

	// Connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("[DB-POOL] Failed to get underlying DB connection: %v", err)
	}
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)

	log.Println("[DB-POOL] Connection pool configured")
	log.Println("[DB-STATUS] Database setup complete")
	return db
}

// Pluralize converts singular resource names to plural
func Pluralize(singular string) string {
	// Handle special cases
	specialCases := map[string]string{
		"category": "categories",
		"company":  "companies",
		"city":     "cities",
	}

	if plural, exists := specialCases[singular]; exists {
		return plural
	}

	// Default pluralization rules
	if strings.HasSuffix(singular, "s") ||
		strings.HasSuffix(singular, "sh") ||
		strings.HasSuffix(singular, "ch") ||
		strings.HasSuffix(singular, "x") ||
		strings.HasSuffix(singular, "z") {
		return singular + "es"
	}

	if strings.HasSuffix(singular, "y") && len(singular) > 1 {
		// Check if the letter before 'y' is a consonant
		beforeY := singular[len(singular)-2]
		if beforeY != 'a' && beforeY != 'e' && beforeY != 'i' && beforeY != 'o' && beforeY != 'u' {
			return singular[:len(singular)-1] + "ies"
		}
	}

	// Default: just add 's'
	return singular + "s"
}

// getActionDescription returns descriptive text for each CRUD action
func getActionDescription(action, resource string) string {
	// Remove 's' from resource for singular form in descriptions
	singularResource := strings.TrimSuffix(resource, "s")

	switch action {
	case "create":
		return fmt.Sprintf("Create new %s", resource)
	case "read":
		return fmt.Sprintf("View %s information", singularResource)
	case "update":
		return fmt.Sprintf("Update %s information", singularResource)
	case "delete":
		return fmt.Sprintf("Delete %s", resource)
	default:
		// Create a title caser for the default case
		caser := cases.Title(language.English)
		return fmt.Sprintf("%s %s", caser.String(action), resource)
	}
}

// GenerateModelPermissions creates CRUD permissions for a model/resource
func GenerateModelPermissions(db *gorm.DB, resourceName, category, createdBy string) error {
	actions := []string{"create", "read", "update", "delete"}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, action := range actions {
			permissionName := fmt.Sprintf("%s.%s", strings.TrimSuffix(resourceName, "s"), action)
			description := getActionDescription(action, resourceName)

			// Use FirstOrCreate for better upsert logic
			permission := models.Permission{
				Name:        permissionName,
				Description: description,
				Resource:    resourceName,
				Action:      action,
				Category:    category,
				IsActive:    true,
				CreatedBy:   createdBy,
			}

			var existingPermission models.Permission
			result := tx.Where("name = ?", permissionName).FirstOrCreate(&existingPermission, permission)

			if result.Error != nil {
				return fmt.Errorf("failed to create/find permission %s: %w", permissionName, result.Error)
			}

			// If record existed (RowsAffected = 0), update it with current values
			if result.RowsAffected == 0 {
				updates := map[string]interface{}{
					"description": description,
					"resource":    resourceName,
					"action":      action,
					"category":    category,
					"is_active":   true,
				}

				if err := tx.Model(&existingPermission).Updates(updates).Error; err != nil {
					return fmt.Errorf("failed to update permission %s: %w", permissionName, err)
				}
			}
		}
		return nil
	})
}

// GetPermissionsByResource returns all permissions for a specific resource
func GetPermissionsByResource(db *gorm.DB, resource string) ([]models.Permission, error) {
	var permissions []models.Permission
	err := db.Where("resource = ?", resource).Find(&permissions).Error
	return permissions, err
}

// GetAllPermissions returns all permissions in the system
func GetAllPermissions(db *gorm.DB) ([]models.Permission, error) {
	var permissions []models.Permission
	err := db.Order("resource, action").Find(&permissions).Error
	return permissions, err
}

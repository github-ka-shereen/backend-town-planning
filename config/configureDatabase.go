package config

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"
	"town-planning-backend/db/models"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var allModels = []interface{}{
	// 1. Core Authentication and Authorization Models
	&models.Permission{},
	&models.Role{},
	&models.RolePermission{},
	&models.Department{},
	&models.User{},
	&models.UserAuditLog{},

	// 2. Document Management Models (standalone categories first)
	&models.DocumentCategory{},

	// 3. Property and Stand Management Models
	&models.DevelopmentCategory{},
	&models.StandType{},
	&models.Project{},
	&models.Stand{},

	// 4. Applicant Models (before Application)
	&models.Applicant{},
	&models.ApplicantAdditionalPhone{},
	&models.OrganisationRepresentative{},
	&models.ApplicantOrganisationRepresentative{},

	// 5. Financial Models
	&models.Tariff{},
	&models.VATRate{},

	// 5a. Bank / Forex models (needed for Payment)
	&models.Bank{},
	&models.BankAccount{},
	&models.ExchangeRate{},

	// 6. Permit models (AFTER Application since it references Application)
	&models.Permit{},
	&models.Application{},

	// 6a. Payment tracking
	&models.Payment{},

	// 7. Document models (now all referenced tables exist)
	&models.Document{},
	&models.DocumentAuditLog{},
	&models.DocumentVersion{},

	// 8. Other models that reference the above
	&models.Comment{},
	&models.ApplicantDocument{},
	&models.AllStandOwners{},
	&models.EmailLog{},
	&models.BulkUploadErrorProjects{},
	&models.BulkUploadErrorStands{},
	&models.BulkStandUploadError{},
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
	// 	// "users": true,
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

	// ===== END Drop all tables =====


	// ===== Drop specific tables =====

	// If you need to drop specific tables in a particular order (like join tables first),
	// you can add that logic here:

	// // Drop join tables first to avoid foreign key constraints
	// if err := db.Migrator().DropTable("stand_swap_owners"); err != nil {
	// 	log.Printf("Failed to drop stand_swap_owners join table: %v", err)
	// }

	// // Then drop other specific tables
	// specificTablesToDrop := []string{
	// 	"applications",
	// }

	// for _, table := range specificTablesToDrop {
	// 	if err := db.Migrator().DropTable(table); err != nil {
	// 		log.Printf("Failed to drop table %s: %v", table, err)
	// 	}
	// }

	// ===== END Drop specific tables =====

	// // ===== END OF CLEANUP CODE =====

	// Auto-migrate all models using the allModels slice
	err = db.AutoMigrate(allModels...)
	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	} else {
		log.Println("Tables migrated successfully")
	}

	// ===== Auto-generate permissions for all models =====
	// // Simplified manual skip list - only for special non-join tables
	// skipList := map[string]bool{
	// 	"permissions":     true, // Don't generate permissions for permissions themselves
	// 	"user_audit_logs": true, // Audit table (not a join table but should be skipped)
	// 	// Add other non-join tables to skip here
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

	// 	// Check if this is a join table using improved detection and skip it
	// 	if isJoinTable(modelType) {
	// 		log.Printf("[PERMISSIONS] 🔗 Skipping join table: %s (no permissions needed)", tableName)
	// 		continue
	// 	}

	// 	// Also check the manual skip list for non-join tables that should be skipped
	// 	if skipList[tableName] {
	// 		log.Printf("[PERMISSIONS] Skipping table (manual skip list): %s", tableName)
	// 		continue
	// 	}

	// 	// Use the model struct name as the resource (e.g., "User" -> "users")
	// 	resource := Pluralize(strings.ToLower(modelType.Name()))
	// 	category := strings.ToLower(modelType.Name()) + "_management"

	// 	if err := GenerateModelPermissions(db, resource, category, "system"); err != nil {
	// 		log.Printf("[PERMISSIONS] Failed to generate permissions for %s: %v", resource, err)
	// 	} else {
	// 		log.Printf("[PERMISSIONS] ✅ Generated CRUD permissions for: %s", resource)
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

// isJoinTable comprehensively detects join tables using multiple criteria
func isJoinTable(structType reflect.Type) bool {
	if structType.Kind() != reflect.Struct {
		return false
	}

	structName := structType.Name()

	// Method 1: Name-based detection (most reliable for conventional naming)
	if isJoinTableByName(structName) {
		return true
	}

	// Method 2: Structural analysis
	if isJoinTableByStructure(structType) {
		return true
	}

	return false
}

// isJoinTableByName detects join tables by naming conventions
func isJoinTableByName(structName string) bool {
	name := strings.ToLower(structName)

	// Pattern 1: Contains common join table keywords
	joinKeywords := []string{
		"permission", "assignment", "mapping", "relation", "link",
		"association", "connection", "junction", "bridge", "pivot",
	}

	for _, keyword := range joinKeywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}

	// Pattern 2: Two entity names combined (e.g., UserRole, ProductCategory)
	// This is harder to detect generically, but we can look for common patterns
	commonEntities := []string{
		"user", "role", "permission", "group", "team", "department",
		"product", "category", "tag", "order", "item", "customer",
		"company", "project", "task", "document", "file", "applicant",
		"organisation", "application", "representative",
	}

	var entityMatches int
	for _, entity := range commonEntities {
		if strings.Contains(name, entity) {
			entityMatches++
		}
	}

	// If name contains 2+ entity names, likely a join table
	if entityMatches >= 2 {
		return true
	}

	return false
}

// isJoinTableByStructure analyzes the struct fields to detect join table patterns
func isJoinTableByStructure(structType reflect.Type) bool {
	var foreignKeyCount int
	var actualDbFieldCount int
	var hasID bool

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldName := field.Name
		fieldType := field.Type

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Skip GORM relationship fields (struct types that aren't DB columns)
		if isRelationshipField(field) {
			continue
		}

		// Count actual database fields
		actualDbFieldCount++

		// Check for ID field
		if fieldName == "ID" {
			hasID = true
			continue
		}

		// Check for timestamp fields
		if isTimestampField(fieldName, fieldType) {
			continue
		}

		// Check for foreign key fields
		if isForeignKeyField(fieldName, fieldType) {
			foreignKeyCount++
		}
	}

	// Join table structural criteria:
	// 1. Has exactly 2 foreign keys (the core requirement)
	// 2. Low total DB field count (ID + 2 FKs + maybe timestamps = 3-6 fields typically)
	// 3. Usually has ID and timestamps
	return foreignKeyCount == 2 &&
		actualDbFieldCount >= 3 && actualDbFieldCount <= 8 &&
		hasID
}

// Helper functions for join table detection

func isRelationshipField(field reflect.StructField) bool {
	fieldType := field.Type

	// Direct struct types (belongs-to relationships)
	if fieldType.Kind() == reflect.Struct && !isPrimitiveStructType(fieldType) {
		return true
	}

	// Pointer to struct (optional belongs-to)
	if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct && !isPrimitiveStructType(fieldType.Elem()) {
		return true
	}

	// Slices (has-many relationships)
	if fieldType.Kind() == reflect.Slice {
		elemType := fieldType.Elem()
		if elemType.Kind() == reflect.Struct && !isPrimitiveStructType(elemType) {
			return true
		}
	}

	return false
}

func isPrimitiveStructType(fieldType reflect.Type) bool {
	typeName := fieldType.String()
	primitiveStructs := []string{
		"time.Time", "uuid.UUID", "gorm.DeletedAt",
		"datatypes.JSON", "datatypes.Date", "datatypes.Time",
	}

	for _, primitive := range primitiveStructs {
		if typeName == primitive {
			return true
		}
	}

	return false
}

func isForeignKeyField(fieldName string, fieldType reflect.Type) bool {
	// Must end with "ID" but not be the primary key "ID"
	if !strings.HasSuffix(fieldName, "ID") || fieldName == "ID" {
		return false
	}

	// Should be a UUID type for foreign keys
	return fieldType.String() == "uuid.UUID" || fieldType.String() == "*uuid.UUID"
}

func isTimestampField(fieldName string, fieldType reflect.Type) bool {
	timestampNames := []string{"CreatedAt", "UpdatedAt", "DeletedAt"}
	for _, name := range timestampNames {
		if fieldName == name {
			return true
		}
	}
	return fieldType.String() == "time.Time" || fieldType.String() == "*time.Time" || fieldType.String() == "gorm.DeletedAt"
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

			// Check if permission already exists
			var existingPermission models.Permission
			err := tx.Where("name = ?", permissionName).First(&existingPermission).Error

			if err == nil {
				// Permission exists - skip creation to preserve role assignments
				log.Printf("[PERMISSIONS] ↻ Permission already exists, skipping: %s", permissionName)
				continue
			} else if err != gorm.ErrRecordNotFound {
				// Actual database error
				return fmt.Errorf("failed to check existing permission %s: %w", permissionName, err)
			}

			// Permission doesn't exist - create it
			description := getActionDescription(action, resourceName)
			permission := models.Permission{
				Name:        permissionName,
				Description: description,
				Resource:    resourceName,
				Action:      action,
				Category:    category,
				IsActive:    true,
				CreatedBy:   createdBy,
			}

			if err := tx.Create(&permission).Error; err != nil {
				return fmt.Errorf("failed to create permission %s: %w", permissionName, err)
			}

			log.Printf("[PERMISSIONS] ✨ Created new permission: %s", permissionName)
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

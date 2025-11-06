package config

import (
	"fmt"
	"log"
	"town-planning-backend/db/models"

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

	// 6. Core Application and Permit models
	&models.Application{},
	&models.Permit{},

	// 6a. Payment tracking
	&models.Payment{},

	// 7. Document models (now all referenced tables exist)
	&models.Document{},
	&models.DocumentAuditLog{},

	// 8. Approval Workflow Models
	&models.ApprovalGroup{},
	&models.ApprovalGroupMember{},
	&models.ApplicationGroupAssignment{},
	&models.MemberApprovalDecision{},
	&models.ApplicationIssue{},
	&models.FinalApproval{},
	&models.Comment{},

	// 9. NEW: Chat System Models (add before document join tables)
	&models.ChatThread{},
	&models.ChatParticipant{},
	&models.ChatMessage{},
	&models.ReadReceipt{},
	&models.ChatAttachment{},

	// 10. Document Join Tables (must come after Document and related entities)
	&models.ApplicantDocument{},
	&models.ApplicationDocument{},
	&models.StandDocument{},
	&models.ProjectDocument{},
	&models.CommentDocument{},
	&models.PaymentDocument{},
	&models.EmailDocument{},
	&models.BankDocument{},
	&models.UserDocument{},
	&models.ChatAttachment{}, // This references Document, so it should be here

	// 11. Other models that reference the above
	&models.AllStandOwners{},
	&models.Reservation{},
	&models.EmailLog{},

	// 12. Bulk Upload Error Models
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

	// ===== Drop all tables =====

	// // Define a skip list for tables that should not be dropped
	// skipTables := map[string]bool{
	// 	// 	"users": true,
	// }

	// // Drop all tables EXCEPT skipTables
	// for _, table := range tables {
	// 	if skipTables[table] {
	// 		continue // Skip tables in the skip list
	// 	}
	// 	if err := db.Migrator().DropTable(table); err != nil {
	// 		log.Fatalf("Failed to drop table %s: %v", table, err)
	// 	}
	// }

	// Auto-migrate all models using the allModels slice
	err = db.AutoMigrate(allModels...)
	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	} else {
		log.Println("Tables migrated successfully")
	}

	return db
}

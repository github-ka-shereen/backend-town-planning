package config

import (
	// "town-planning-backend/db/models"

	"fmt"
	"log"
	"time"
	"town-planning-backend/db/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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

	// // Drop only the StandSwap table
	// if err := db.Migrator().DropTable("stand_swap_owners"); err != nil {
	// 	log.Fatalf("Failed to drop stand_swap_owners join table: %v", err)
	// }

	// // 2. Then, drop the StandSwap table.
	// //    Now that the join table is gone, there are no foreign keys
	// //    pointing to StandSwap, so it can be dropped safely.
	// if err := db.Migrator().DropTable(
	// 	&models.CommunicationRecipient{},
	// 	&models.CommunicationAttachment{},
	// 	&models.CommunicationGroup{},
	// 	&models.Communication{},
	// 	); err != nil {
	// 	log.Fatalf("Failed to drop tables: %v", err)
	// }

	// // Define a skip list for tables that should not be dropped
	// skipTables := map[string]bool{
	// 	// "users": true,
	// 	// "clients": true,
	// 	// Add other tables to skip here
	// 	// "email_logs": true,
	// }

	// // Drop all tables EXCEPT skipTables
	// for _, table := range tables {
	// 	// if skipTables[table] {
	// 	// 	continue // Skip tables in the skip list
	// 	// }
	// 	if err := db.Migrator().DropTable(table); err != nil {
	// 		log.Fatalf("Failed to drop table %s: %v", table, err)
	// 	}
	// }

	// Auto-migrate all models (including users table which was preserved)
	err = db.AutoMigrate(
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
	)

	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	} else {
		log.Println("Tables migrated successfully")
	}

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

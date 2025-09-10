package config

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func RestoreDatabase() error {
	password := GetEnv("POSTGRES_PASSWORD")
	dbName := GetEnv("POSTGRES_DB")
	user := GetEnv("POSTGRES_USER")

	// Terminate other sessions
	terminateCmd := fmt.Sprintf("PGPASSWORD=%s docker exec -i backend-db_acrepoint-1 psql -U %s postgres -c \"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid();\"", password, user, dbName)
	execCmd := exec.Command("bash", "-c", terminateCmd)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error terminating other sessions: %v", err)
		log.Println(string(output))
		return err
	}

	// Drop the database
	dropCmd := fmt.Sprintf("PGPASSWORD=%s docker exec -i backend-db_acrepoint-1 psql -U %s postgres -c 'DROP DATABASE IF EXISTS %s;'", password, user, dbName)
	execCmd = exec.Command("bash", "-c", dropCmd)
	output, err = execCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error dropping database: %v", err)
		log.Println(string(output))
		return err
	}

	// Create the database
	createCmd := fmt.Sprintf("PGPASSWORD=%s docker exec -i backend-db_acrepoint-1 psql -U %s -d postgres -c 'CREATE DATABASE %s;'", password, user, dbName)
	execCmd = exec.Command("bash", "-c", createCmd)
	output, err = execCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error creating database: %v", err)
		log.Println(string(output))
		return err
	}

	// Restore the database
	restoreCmd := fmt.Sprintf("PGPASSWORD=%s docker exec -i backend-db_acrepoint-1 psql -U %s %s", password, user, dbName)
	execCmd = exec.Command("bash", "-c", restoreCmd)
	file, err := os.Open("./db_backup_2025-06-17_23-44-06.sql")
	if err != nil {
		log.Printf("Error opening database backup file: %v", err)
		return err
	}
	defer file.Close()
	execCmd.Stdin = file
	output, err = execCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error restoring database: %v", err)
		log.Println(string(output))
		return err
	}

	log.Println("Database restore successful")
	return nil
}

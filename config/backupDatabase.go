package config

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func BackupDatabase() error {
	password := GetEnv("POSTGRES_PASSWORD")
	cmd := fmt.Sprintf("PGPASSWORD=%s docker exec -i backend-db_acrepoint-1 pg_dump -U %s %s", password, GetEnv("POSTGRES_USER"), GetEnv("POSTGRES_DB"))
	execCmd := exec.Command("bash", "-c", cmd)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error backing up database: %v", err)
		log.Println(string(output))
		return err
	}
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("./db_backup_%s.sql", timestamp)
	err = os.WriteFile(fileName, output, 0644)
	if err != nil {
		log.Printf("Error writing database backup to file: %v", err)
		return err
	}
	log.Printf("Database backup successful: %s", fileName)
	return nil
}

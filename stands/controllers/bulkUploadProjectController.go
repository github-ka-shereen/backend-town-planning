package controllers

import (
	"fmt"
	"os"
	"time"

	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/stands/services"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

type ErrorDetails struct {
	ProjectNumber string `json:"project_number"`
	ProjectName   string `json:"project_name"`
	Address       string `json:"address"`
	City          string `json:"city"`
	Reason        string `json:"reason"`
	ErrorType     string `json:"error_type"`
	AddedVia      string `json:"added_via"`
	CreatedBy     string `json:"created_by"`
}

// BulkUpload handles the bulk upload of projects via an Excel file.
func (sc *StandController) BulkUploadProjects(c *fiber.Ctx) error {
	// Extract the uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Failed to get file"})
	}

	tempFilePath := fmt.Sprintf("./tmp/%s", file.Filename)
	if err := c.SaveFile(file, tempFilePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to save file"})
	}
	defer os.Remove(tempFilePath)

	// Extract the 'created_by' field from FormData
	userEmail := c.FormValue("created_by")
	if userEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Missing 'created_by' field in FormData"})
	}

	// Open and read the Excel file
	f, err := excelize.OpenFile(tempFilePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to open Excel file"})
	}
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to read rows from Excel sheet"})
	}

	// Process rows and populate projects
	var projectsToProcess []models.Project // Renamed to avoid confusion with `filteredProjects` later
	var projectNumbersInFile []string      // Track project numbers seen in this file
	var invalidRows []models.BulkUploadErrorProjects
	var duplicateErrors []models.BulkUploadErrorProjects // Duplicates already in DB or within file

	// Map to detect duplicates within the uploaded file
	projectNumbersInFileMap := make(map[string]struct{})

	for i, row := range rows {
		if i == 0 { // Skip the header row
			continue
		}

		// Check that the row has at least 4 columns (ProjectNumber, ProjectName, Address, City)
		if len(row) < 4 {
			invalidRows = append(invalidRows, models.BulkUploadErrorProjects{
				ID:            uuid.New(),
				ProjectNumber: "", // Can't reliably get if columns are too few
				ProjectName:   "",
				Address:       "",
				City:          "",
				CreatedBy:     userEmail,
				Reason:        fmt.Sprintf("row %d has insufficient columns", i+1),
				ErrorType:     models.MissingDataErrorType, // ALLOWED ERROR
				AddedVia:      models.BulkAddedViaType,
			})
			continue // Skip to next row, this is an allowed error
		}

		projectNumberFromRow := row[0]
		// Check for duplicates within the uploaded file itself
		if _, exists := projectNumbersInFileMap[projectNumberFromRow]; exists {
			duplicateErrors = append(duplicateErrors, models.BulkUploadErrorProjects{
				ID:            uuid.New(),
				ProjectNumber: projectNumberFromRow,
				ProjectName:   row[1],
				Address:       row[2],
				City:          row[3],
				Reason:        fmt.Sprintf("Duplicate project number in the uploaded file in row %d", i+1),
				CreatedBy:     userEmail,
				AddedVia:      models.BulkAddedViaType,
				ErrorType:     models.DuplicateErrorType, // ALLOWED ERROR
			})
			continue // Skip to next row, this is an allowed error
		}
		projectNumbersInFileMap[projectNumberFromRow] = struct{}{}

		// Pass userEmail to ValidateProjectRow
		project, err := services.ValidateProjectRow(row, i, userEmail)
		if err != nil {
			// This handles validation errors from ValidateProjectRow, which are allowed errors.
			projectNumber := row[0]
			projectName := row[1]
			address := row[2]
			city := row[3]

			invalidRows = append(invalidRows, models.BulkUploadErrorProjects{
				ID:            uuid.New(),
				ProjectNumber: projectNumber,
				ProjectName:   projectName,
				Address:       address,
				City:          city,
				CreatedBy:     userEmail,
				Reason:        err.Error(),
				ErrorType:     models.MissingDataErrorType, // ALLOWED ERROR
				AddedVia:      models.BulkAddedViaType,
			})
			continue // Skip to next row, this is an allowed error
		}

		// Add the valid project to the list
		projectsToProcess = append(projectsToProcess, project)
		projectNumbersInFile = append(projectNumbersInFile, project.ProjectNumber)
	}

	// Check database for duplicate project numbers (before transaction)
	existingDBDuplicates, err := sc.StandRepo.FindDuplicateProjectNumbers(projectNumbersInFile)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to check for existing duplicate project numbers in database"})
	}

	// Add existing database duplicates to duplicateErrors
	for _, dupNum := range existingDBDuplicates {
		for _, project := range projectsToProcess {
			if project.ProjectNumber == dupNum {
				duplicateErrors = append(duplicateErrors, models.BulkUploadErrorProjects{
					ID:            uuid.New(),
					ProjectNumber: dupNum,
					ProjectName:   project.ProjectName,
					Address:       project.Address,
					City:          project.City,
					Reason:        "Project number already exists in the database",
					CreatedBy:     userEmail,
					AddedVia:      models.BulkAddedViaType,
					ErrorType:     models.DuplicateErrorType, // ALLOWED ERROR
				})
				break
			}
		}
	}

	// Filter out both file-level duplicates and database duplicates before attempting creation
	filteredProjects := []models.Project{}
	for _, project := range projectsToProcess {
		isDuplicate := false
		for _, dupErr := range duplicateErrors {
			if project.ProjectNumber == dupErr.ProjectNumber {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			filteredProjects = append(filteredProjects, project)
		}
	}

	var downloadLink *string
	// Combine all errors for the report
	allErrorRecords := append(invalidRows, duplicateErrors...)

	// Log invalid/duplicate rows and generate error report
	if len(allErrorRecords) > 0 {
		// Log all types of errors to the database
		// Assuming LogBulkUploadErrors can handle both types or you have separate methods
		if err := sc.StandRepo.LogBulkUploadErrors(invalidRows); err != nil {
			config.Logger.Error("Warning: Failed to log invalid rows for projects", zap.Error(err))
		}
		if err := sc.StandRepo.LogDuplicateProjects(duplicateErrors); err != nil {
			config.Logger.Error("Warning: Failed to log duplicate projects", zap.Error(err))
		}

		headers := []string{"ProjectNumber", "ProjectName", "Address", "City", "Reason", "ErrorType", "AddedVia", "CreatedBy"}
		queryHash := uuid.New().String()
		filePath, err := utils.GenerateExcel(allErrorRecords, queryHash, headers)
		if err != nil {
			config.Logger.Error("Warning: Failed to generate Excel for project errors", zap.Error(err))
		} else {
			link := utils.GenerateDownloadLink(filePath)
			downloadLink = &link

			message := "Please find the attached file with error records (missing fields and duplicates)."
			subject := "Project Upload Errors - " + time.Now().Format("2006-01-02 15:04:05")

			// Send the email
			err := utils.SendEmail(userEmail, message, subject, "", *downloadLink)
			if err != nil {
				config.Logger.Error("Warning: Failed to send email with project error report", zap.Error(err))
			} else {
				active := true
				// Log the email sent to the database
				emailLog := models.EmailLog{
					ID:             uuid.New().String(),
					Recipient:      userEmail,
					Subject:        subject,
					Message:        message,
					SentAt:         utils.Today(),
					Active:         &active,
					AttachmentPath: *downloadLink,
				}
				if err := sc.StandRepo.LogEmailSent(&emailLog); err != nil {
					config.Logger.Error("Warning: Failed to log email for project errors", zap.Error(err))
				}
			}
		}
	}

	// --- Start Database Transaction for valid projects ---
	if len(filteredProjects) > 0 {
		tx := sc.DB.Begin()
		if tx.Error != nil {
			config.Logger.Error("Failed to begin database transaction for projects", zap.Error(tx.Error))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to begin database transaction", "error": tx.Error.Error()})
		}

		// Pass the transaction object to the repository's bulk create method
		err = sc.StandRepo.BulkCreateProjects(tx, filteredProjects)
		if err != nil {
			tx.Rollback() // Rollback all changes if bulk creation fails
			config.Logger.Error("Critical: Transaction rolled back due to BulkCreateProjects error", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message":            fmt.Sprintf("Failed to insert projects: %v. Database changes rolled back.", err.Error()),
				"successful_count":   0, // No projects committed
				"duplicates_count":   len(duplicateErrors),
				"missing_data_count": len(invalidRows),
				"error_file_path":    downloadLink,
			})
		}

		// Commit the transaction if all projects were successfully created
		if err := tx.Commit().Error; err != nil {
			tx.Rollback() // In case commit itself fails, try to rollback
			config.Logger.Error("Critical: Transaction rolled back due to commit error for projects", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message":            fmt.Sprintf("Failed to commit project insertions: %v. Database changes rolled back.", err.Error()),
				"successful_count":   0,
				"duplicates_count":   len(duplicateErrors),
				"missing_data_count": len(invalidRows),
				"error_file_path":    downloadLink,
			})
		}

		// --- Index projects in Bleve Search (only after successful DB commit) ---
		if sc.BleveRepo != nil {
			for _, projectIndex := range filteredProjects {
				if err := sc.BleveRepo.IndexSingleProject(projectIndex); err != nil {
					// Log the error but continue with other projects.
					// Bleve indexing failures do not trigger a DB rollback as it's often eventually consistent.
					config.Logger.Error("Warning: Error indexing project in Bleve", zap.Error(err), zap.String("projectID", projectIndex.ID.String()))
				} else {
					config.Logger.Info("Successfully indexed project in Bleve", zap.String("projectID", projectIndex.ID.String()), zap.Any("Project", projectIndex))
				}
			}
		} else {
			config.Logger.Warn("BleveRepo is nil, skipping document indexing for projects")
		}
	}

	// Return response with duplicates and successful uploads
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":            "Bulk upload completed",
		"successful_count":   len(filteredProjects),
		"duplicates_count":   len(duplicateErrors), // Total duplicates (in file + in DB)
		"missing_data_count": len(invalidRows),     // Total missing data errors
		"error_file_path":    downloadLink,
	})
}

// Helper function to check if a slice contains a specific project number
// func contains(slice []string, item string) bool {
// 	for _, v := range slice {
// 		if v == item {
// 			return true
// 		}
// 	}
// 	return false
// }

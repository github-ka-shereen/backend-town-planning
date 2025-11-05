package controllers

import (
	"fmt"
	"log"
	"os"
	"time" // Needed for time.Now() if used elsewhere, though not explicitly in this snippet

	"town-planning-backend/db/models"
	"town-planning-backend/stands/services"
	"town-planning-backend/utils" // Ensure this includes GenerateExcel, GenerateDownloadLink, SendEmail, Today

	// Assuming your StandRepo and ProjectRepo are defined here
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	// Import GORM for transaction management
)

type StandErrorDetails struct {
	StandNumber   string               `json:"stand_number"`
	StandCost     decimal.Decimal      `json:"stand_cost"`
	StandSize     decimal.Decimal      `json:"stand_size"`
	StandCurrency models.StandCurrency `json:"stand_currency"`
	StandType     models.StandType     `json:"stand_type"`
	ProjectNumber string               `json:"project_number"`
	Reason        string               `json:"reason"`
	ErrorType     string               `json:"error_type"`
	AddedVia      string               `json:"added_via"`
	CreatedBy     string               `json:"created_by"`
}

// BulkUpload handles the bulk upload of stands via an Excel file.
func (sc *StandController) BulkUploadStands(c *fiber.Ctx) error {
	// Parse and save the uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Failed to get file"})
	}
	tempFilePath := fmt.Sprintf("./tmp/%s", file.Filename)
	err = c.SaveFile(file, tempFilePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to save file"})
	}
	defer os.Remove(tempFilePath)

	// Extract the 'created_by' field from FormData
	userEmail := c.FormValue("created_by")
	if userEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Missing 'created_by' field in FormData"})
	}

	// Get VAT flag from form data (default to true if not provided)
	applyVAT := true
	vatFlag := c.FormValue("apply_vat")
	if vatFlag == "false" {
		applyVAT = false
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

	// --- Fetch VAT Rate Once for the entire bulk upload ---
	// --- Fetch VAT Rate Only if Needed ---
	var vatRate decimal.Decimal
	if applyVAT {
		vatRateRecord, err := sc.StandRepo.GetActiveVATRate()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to retrieve active VAT rate for bulk upload",
				"data":    nil,
				"error":   err.Error(),
			})
		}
		if vatRateRecord == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "No active VAT rate found for bulk upload calculation. Please configure VAT rate.",
				"data":    nil,
				"error":   "VAT rate not configured",
			})
		}
		vatRate = vatRateRecord.Rate
	}
	// --- End VAT Rate Fetch ---

	var validStands []models.Stand
	var invalidRows []models.BulkUploadErrorStands
	standNumbersInFile := make(map[string]struct{}) // Track duplicates in the *current batch*

	for i, row := range rows {
		if i == 0 { // Skip header row
			continue
		}

		// Declare projectNumber variable here
		var projectNumber string

		// Extract projectNumber from row (optional)
		if len(row) > 5 {
			projectNumber = row[5]
		}
		if projectNumber == "" {
			projectNumber = "no project number" // Default value if missing
		}

		standNumber := row[0]
		// Check for duplicates within the uploaded file
		if _, exists := standNumbersInFile[standNumber]; exists {
			taxExclusiveStandPriceFromRow, _ := decimal.NewFromString(row[1])
			var vatAmountForError decimal.Decimal
			var standCostForError decimal.Decimal

			if applyVAT {
				vatAmountForError = taxExclusiveStandPriceFromRow.Mul(vatRate)
				standCostForError = taxExclusiveStandPriceFromRow.Add(vatAmountForError)
			} else {
				vatAmountForError = decimal.Zero
				standCostForError = taxExclusiveStandPriceFromRow
			}

			size, _ := decimal.NewFromString(row[2])

			invalidRows = append(invalidRows, models.BulkUploadErrorStands{
				ID:                     uuid.New(),
				StandNumber:            standNumber,
				TaxExclusiveStandPrice: taxExclusiveStandPriceFromRow,
				VATAmount:              vatAmountForError,
				StandCost:              standCostForError,
				StandSize:              size,
				StandCurrency:          models.StandCurrency(row[3]),
				StandTypeName:          row[4],
				ProjectNumber:          row[5],
				Reason:                 fmt.Sprintf("Duplicate stand number in the uploaded file in row %d", i+1),
				ErrorType:              models.DuplicateErrorType,
				AddedVia:               models.BulkAddedViaType,
				CreatedBy:              userEmail,
			})
			continue
		}
		standNumbersInFile[standNumber] = struct{}{}

		// Validate individual stand row
		stand, err := services.ValidateStandRow(row, i, sc.StandRepo, userEmail)
		if err != nil {
			if projectNotFoundErr, ok := err.(*services.ProjectNotFoundError); ok {
				errorStand := projectNotFoundErr.BulkUploadError
				taxExclusiveProvided := !errorStand.TaxExclusiveStandPrice.IsZero()
				standCostProvided := !errorStand.StandCost.IsZero()

				if applyVAT {
					if taxExclusiveProvided {
						errorStand.VATAmount = errorStand.TaxExclusiveStandPrice.Mul(vatRate)
						errorStand.StandCost = errorStand.TaxExclusiveStandPrice.Add(errorStand.VATAmount)
					} else if standCostProvided {
						onePlusVATRate := decimal.NewFromInt(1).Add(vatRate)
						if !onePlusVATRate.IsZero() {
							errorStand.TaxExclusiveStandPrice = errorStand.StandCost.Div(onePlusVATRate)
							errorStand.VATAmount = errorStand.StandCost.Sub(errorStand.TaxExclusiveStandPrice)
						}
					}
				} else {
					if taxExclusiveProvided {
						errorStand.VATAmount = decimal.Zero
						errorStand.StandCost = errorStand.TaxExclusiveStandPrice
					} else if standCostProvided {
						errorStand.VATAmount = decimal.Zero
						errorStand.TaxExclusiveStandPrice = errorStand.StandCost
					}
				}
				invalidRows = append(invalidRows, errorStand)
			} else {
				taxExclusiveStandPriceForError, _ := decimal.NewFromString(row[1])
				var vatAmountForError decimal.Decimal
				var standCostForError decimal.Decimal

				if applyVAT {
					vatAmountForError = taxExclusiveStandPriceForError.Mul(vatRate)
					standCostForError = taxExclusiveStandPriceForError.Add(vatAmountForError)
				} else {
					vatAmountForError = decimal.Zero
					standCostForError = taxExclusiveStandPriceForError
				}

				sizeForError, _ := decimal.NewFromString(row[2])

				invalidRows = append(invalidRows, models.BulkUploadErrorStands{
					ID:                     uuid.New(),
					StandNumber:            standNumber,
					TaxExclusiveStandPrice: taxExclusiveStandPriceForError,
					VATAmount:              vatAmountForError,
					StandCost:              standCostForError,
					StandSize:              sizeForError,
					StandCurrency:          models.StandCurrency(row[3]),
					StandTypeName:          row[4],
					ProjectNumber:          row[5],
					Reason:                 err.Error(),
					CreatedBy:              userEmail,
					ErrorType:              models.MissingDataErrorType,
					AddedVia:               models.BulkAddedViaType,
				})
			}
			continue
		}

		// VAT Calculation Logic
		taxExclusiveProvided := !stand.TaxExclusiveStandPrice.IsZero()
		standCostProvided := !stand.StandCost.IsZero()

		if applyVAT {
			if taxExclusiveProvided && standCostProvided {
				stand.VATAmount = stand.TaxExclusiveStandPrice.Mul(vatRate)
				stand.StandCost = stand.TaxExclusiveStandPrice.Add(stand.VATAmount)
			} else if taxExclusiveProvided {
				stand.VATAmount = stand.TaxExclusiveStandPrice.Mul(vatRate)
				stand.StandCost = stand.TaxExclusiveStandPrice.Add(stand.VATAmount)
			} else if standCostProvided {
				onePlusVATRate := decimal.NewFromInt(1).Add(vatRate)
				if onePlusVATRate.IsZero() {
					log.Printf("Error processing stand %s: Invalid VAT rate (1 + VATRate is zero)", stand.StandNumber)
					invalidRows = append(invalidRows, models.BulkUploadErrorStands{
						ID:                     uuid.New(),
						StandNumber:            stand.StandNumber,
						TaxExclusiveStandPrice: stand.TaxExclusiveStandPrice,
						StandCost:              stand.StandCost,
						StandSize:              stand.StandSize,
						StandCurrency:          stand.StandCurrency,
						StandTypeName:          stand.StandType.Name,
						Reason:                 "Invalid VAT rate for calculation",
						CreatedBy:              userEmail,
						ErrorType:              models.CalculationErrorType,
						AddedVia:               models.BulkAddedViaType,
					})
					continue
				}
				stand.TaxExclusiveStandPrice = stand.StandCost.Div(onePlusVATRate)
				stand.VATAmount = stand.StandCost.Sub(stand.TaxExclusiveStandPrice)
			}
		} else {
			// When VAT is not applied
			if taxExclusiveProvided {
				stand.VATAmount = decimal.Zero
				stand.StandCost = stand.TaxExclusiveStandPrice
			} else if standCostProvided {
				stand.VATAmount = decimal.Zero
				stand.TaxExclusiveStandPrice = stand.StandCost
			}
		}

		if !taxExclusiveProvided && !standCostProvided {
			log.Printf("Error processing stand %s: Neither tax exclusive price nor stand cost provided after validation.", stand.StandNumber)
			invalidRows = append(invalidRows, models.BulkUploadErrorStands{
				ID:            uuid.New(),
				StandNumber:   stand.StandNumber,
				StandSize:     stand.StandSize,
				StandCurrency: stand.StandCurrency,
				StandTypeName: stand.StandType.Name,
				Reason:        "Neither tax exclusive price nor stand cost provided in Excel row",
				CreatedBy:     userEmail,
				ErrorType:     models.MissingDataErrorType,
				AddedVia:      models.BulkAddedViaType,
			})
			continue
		}

		stand.Status = models.UnallocatedStatus
		stand.CreatedBy = userEmail
		validStands = append(validStands, stand)
	}

	// Check database for duplicate stand numbers (before transaction)
	var standNumbersForDBCheck []string
	for standNumber := range standNumbersInFile { // Only check stands that passed initial file validation
		standNumbersForDBCheck = append(standNumbersForDBCheck, standNumber)
	}

	existingDuplicates, err := sc.StandRepo.FindDuplicateStandNumbers(standNumbersForDBCheck)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to check for duplicate stand numbers in database"})
	}

	// Filter out validStands that are actually duplicates in the database
	// And log these as allowed errors.
	filteredValidStands := []models.Stand{}
	dbDuplicateMap := make(map[string]struct{})
	for _, dup := range existingDuplicates {
		dbDuplicateMap[dup] = struct{}{}
		invalidRows = append(invalidRows, models.BulkUploadErrorStands{
			ID:          uuid.New(),
			StandNumber: dup,
			Reason:      "Duplicate stand number already exists in the database",
			ErrorType:   models.DuplicateErrorType, // ALLOWED ERROR
			AddedVia:    models.BulkAddedViaType,
			CreatedBy:   userEmail,
			// Price fields will be zero unless explicitly fetched or determined here.
			TaxExclusiveStandPrice: decimal.Zero,
			VATAmount:              decimal.Zero,
			StandCost:              decimal.Zero,
		})
	}

	for _, stand := range validStands {
		if _, isDBDuplicate := dbDuplicateMap[stand.StandNumber]; !isDBDuplicate {
			filteredValidStands = append(filteredValidStands, stand)
		}
	}
	validStands = filteredValidStands // Update validStands to only include truly new stands

	var downloadLink string

	// Log invalid rows and generate error report
	if len(invalidRows) > 0 {
		if err := sc.StandRepo.LogBulkUploadStandsErrors(invalidRows); err != nil {
			log.Printf("Warning: Failed to log invalid rows: %v", err) // Log but don't fail upload
		}

		headers := []string{"StandNumber", "ProjectNumber", "TaxExclusiveStandPrice", "VATAmount", "StandCost", "StandSize", "StandCurrency", "StandType", "Reason", "ErrorType", "AddedVia", "CreatedBy"}
		queryHash := uuid.New().String()
		filePath, err := utils.GenerateExcel(invalidRows, queryHash, headers)
		if err != nil {
			log.Printf("Warning: Failed to generate error report Excel: %v", err) // Log but don't fail upload
		} else {
			downloadLink = utils.GenerateDownloadLink(filePath)
			message := "Please find the attached file with error records (missing fields and duplicates)."
			subject := "Stand Upload Errors - " + time.Now().Format("2006-01-02 15:04:05")
			err = utils.SendEmail(userEmail, message, subject, "", downloadLink)
			if err != nil {
				log.Printf("Warning: Failed to send email with error report: %v", err) // Log but don't fail upload
			} else {
				active := true
				emailLog := models.EmailLog{
					ID:             uuid.New(),
					Recipient:      userEmail,
					Subject:        subject,
					Message:        message,
					SentAt:         utils.Today(),
					Active:         &active,
					AttachmentPath: downloadLink,
				}
				err = sc.StandRepo.LogEmailSent(&emailLog)
				if err != nil {
					fmt.Println("Warning: Failed to log email:", err) // Log but don't fail upload
				}
			}
		}
	}

	// --- Start Database Transaction for valid stands ---
	if len(validStands) > 0 {
		tx := sc.DB.Begin()
		if tx.Error != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to begin database transaction", "error": tx.Error.Error()})
		}

		// Pass the transaction object to the repository's bulk create method
		err = sc.StandRepo.BulkCreateStands(tx, validStands)
		if err != nil {
			tx.Rollback() // Rollback all changes if bulk creation fails
			log.Printf("Critical: Transaction rolled back due to BulkCreateStands error: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message":            fmt.Sprintf("Failed to insert stands: %v. Database changes rolled back.", err.Error()),
				"successful_count":   0, // No stands committed
				"duplicates_count":   len(existingDuplicates),
				"missing_data_count": len(invalidRows) - len(existingDuplicates), // Adjust count for file/validation errors
				"download_link":      downloadLink,
			})
		}

		// Commit the transaction if all stands were successfully created
		if err := tx.Commit().Error; err != nil {
			tx.Rollback() // In case commit itself fails, try to rollback
			log.Printf("Critical: Transaction rolled back due to commit error: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message":            fmt.Sprintf("Failed to commit stand insertions: %v. Database changes rolled back.", err.Error()),
				"successful_count":   0,
				"duplicates_count":   len(existingDuplicates),
				"missing_data_count": len(invalidRows) - len(existingDuplicates),
				"download_link":      downloadLink,
			})
		}

		// --- Index stands in Bleve Search (only after successful DB commit) ---
		if sc.BleveRepo != nil {
			for _, stand := range validStands {
				if err := sc.BleveRepo.IndexSingleStand(stand); err != nil {
					// Log the error but continue with other stands.
					// Bleve indexing failures do not trigger a DB rollback as it's often eventually consistent.
					log.Printf("Warning: Error indexing stand %s (ID: %s) with Bleve: %v", stand.StandNumber, stand.ID.String(), err)
				}
			}
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":            "Bulk upload completed",
		"successful_count":   len(validStands),
		"duplicates_count":   len(existingDuplicates),                    // This is the count of DB duplicates found
		"missing_data_count": len(invalidRows) - len(existingDuplicates), // Total allowed errors - DB duplicates
		"download_link":      downloadLink,
	})
}

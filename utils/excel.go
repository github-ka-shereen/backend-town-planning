package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/xuri/excelize/v2"
)

// EnsureDirectoryExists ensures the specified directory exists before file saving
func EnsureDirectoryExists(filePath string) error {
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755) // Create the directory with appropriate permissions
		if err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}
	}
	return nil
}

// GenerateExcel creates an Excel file from the provided data
func GenerateExcel(data interface{}, taskName string, headers []string) (string, error) {
	// Ensure the directory exists before attempting to save the file
	dirPath := "./public/files" // This should be where the file will be saved
	err := EnsureDirectoryExists(dirPath)
	if err != nil {
		log.Printf("Failed to ensure directory exists: %v", err)
		return "", fmt.Errorf("failed to ensure directory exists: %v", err)
	}
	log.Printf("Directory exists or created: %s", dirPath)

	// Create a new Excel file
	f := excelize.NewFile()
	log.Println("Created new Excel file")

	// Create a sheet in the Excel file
	sheetName := "Sheet1"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		log.Printf("Error creating sheet: %v", err)
		return "", fmt.Errorf("error creating sheet: %v", err)
	}
	log.Printf("Created sheet: %s (Index: %d)", sheetName, index)

	// Write headers dynamically
	for col, header := range headers {
		cell := fmt.Sprintf("%s1", string(rune(65+col))) // A1, B1, C1, etc.
		err := f.SetCellValue(sheetName, cell, header)
		if err != nil {
			log.Printf("Error setting header %s at %s1: %v", header, string(rune(65+col)), err)
			return "", fmt.Errorf("error setting header %s: %v", header, err)
		}
		log.Printf("Set header %s at %s1", header, string(rune(65+col)))
	}

	// Use reflection to loop over the provided data
	dataSlice := reflect.ValueOf(data)
	if dataSlice.Kind() != reflect.Slice {
		log.Printf("Expected data to be a slice, but got %v", dataSlice.Kind())
		return "", fmt.Errorf("expected data to be a slice")
	}

	// Loop through the data and populate the Excel sheet
	for row := 0; row < dataSlice.Len(); row++ {
		item := dataSlice.Index(row).Interface()

		// Log the row data
		log.Printf("Writing data for row %d", row+2)

		// Loop through the headers to fill the columns
		for col, header := range headers {
			// Use reflection to get the field value of each struct
			field := reflect.ValueOf(item).FieldByName(header)
			if field.IsValid() {
				value := field.Interface()
				cell := fmt.Sprintf("%s%d", string(rune(65+col)), row+2)

				// Log the value being written
				log.Printf("Setting value for field %s (Row: %d, Column: %s): %v", header, row+2, string(rune(65+col)), value)

				// Set the cell value
				err := f.SetCellValue(sheetName, cell, value)
				if err != nil {
					log.Printf("Error setting value for field %s (Row: %d, Column: %s): %v", header, row+2, string(rune(65+col)), err)
					return "", fmt.Errorf("error setting value for field %s (Row: %d, Column: %s): %v", header, row+2, string(rune(65+col)), err)
				}
			} else {
				log.Printf("Field %s not found for row %d", header, row+2)
			}
		}
	}

	// Set the active sheet of the workbook
	f.SetActiveSheet(index)
	log.Println("Set active sheet")

	// Generate filename using taskName and current date and time
	now := time.Now()
	dayOfWeek := now.Weekday().String()
	month := now.Month().String()
	day := now.Day()
	year := now.Year()
	timeFormatted := now.Format("3:04:05 PM")

	// Construct the filename
	fileName := fmt.Sprintf("%s_%s_%s_%dth_%d_at_%s.xlsx",
		taskName, dayOfWeek, month, day, year, timeFormatted)
	filePath := fmt.Sprintf("/public/files/%s", fileName)       // Correctly format the public path
	relativeFilePath := fmt.Sprintf("%s/%s", dirPath, fileName) // Save file in public/files directory

	log.Printf("Saving Excel file to: %s", relativeFilePath)

	// Save the file to the file system
	err = f.SaveAs(relativeFilePath)
	if err != nil {
		log.Printf("Error saving Excel file: %v", err)
		return "", err
	}

	// Successfully saved the file
	log.Printf("Successfully saved the Excel file to: %s", relativeFilePath)

	// Return the path to the saved file
	return filePath, nil
}

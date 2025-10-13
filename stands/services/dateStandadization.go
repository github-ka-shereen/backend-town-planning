package services

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// DateLocation is the application's timezone
var DateLocation *time.Location

// InitializeDateLocation sets up the application's timezone
func InitializeDateLocation() error {
    // Load .env file
    if err := godotenv.Load(); err != nil {
        log.Printf("Warning: Error loading .env file: %v", err)
        // Continue execution as env vars might be set in the system
    }

    timezone := os.Getenv("DB_TIMEZONE")
    if timezone == "" {
        timezone = "Africa/Harare" // fallback default
    }
    
    var err error
    DateLocation, err = time.LoadLocation(timezone)
    return err
}

// NormalizeDate converts a time.Time to a normalized date at midnight in the application timezone
func NormalizeDate(t time.Time) time.Time {
	year, month, day := t.In(DateLocation).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, DateLocation)
}

// Today returns today's date normalized at midnight in the application timezone
func Today() time.Time {
	return NormalizeDate(time.Now())
}

// AreDatesEqual compares two dates, normalizing them first
func AreDatesEqual(date1, date2 time.Time) bool {
	return NormalizeDate(date1).Equal(NormalizeDate(date2))
}

// IsDueToday checks if a due date falls on today
func IsDueToday(dueDate time.Time) bool {
	return AreDatesEqual(dueDate, Today())
}
package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

// Retry configuration
const maxRetries = 3
const retryDelay = 2 * time.Minute // 2 minutes between retries


// CleanupExpiredFiles removes expired files older than the TTL
func CleanupExpiredFiles(filePath string, ttl time.Duration) error {
	// Check if the file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error checking file: %v", err)
	}

	// Check if the file is older than the TTL
	if time.Since(info.ModTime()) > ttl {
		// Delete the file if expired
		err := os.Remove(filePath)
		if err != nil {
			return fmt.Errorf("error deleting expired file: %v", err)
		}
		fmt.Printf("File %s deleted successfully.\n", filePath)
	}
	return nil
}

// CleanupExpiredCache removes expired cache from Redis
func CleanupExpiredCache(redisClient *redis.Client) error {
	cacheKey := "some_unique_cache_key" // dynamic based on your use case
	err := redisClient.Del(context.Background(), cacheKey).Err()
	if err != nil {
		return fmt.Errorf("error deleting cache key from Redis: %v", err)
	}
	fmt.Printf("Cache key %s deleted successfully.\n", cacheKey)
	return nil
}

// CleanupAllExpired handles the cleanup of files and Redis cache entries
func CleanupAllExpired(fileTTL time.Duration, redisClient *redis.Client) error {
	// Example of cleaning expired files
	files, err := os.ReadDir("./public/files")
	if err != nil {
		return fmt.Errorf("error reading files directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Cleanup expired files based on TTL
		filePath := fmt.Sprintf("./public/files/%s", file.Name())
		err := CleanupExpiredFiles(filePath, fileTTL)
		if err != nil {
			fmt.Println("Error cleaning up file:", err)
		}
	}

	// Cleanup Redis cache by passing redisClient
	err = CleanupExpiredCache(redisClient)  // Now passing redisClient
	if err != nil {
		return fmt.Errorf("error cleaning up cache: %v", err)
	}

	return nil
}

// RunScheduledCleanup runs cleanup tasks daily at 1 AM with retries and logs error messages to console on failure
func RunScheduledCleanup(redisClient *redis.Client) {
	// Create a new cron job scheduler
	c := cron.New()

	// Schedule the cleanup task to run every day at 1 AM
	c.AddFunc("0 1 * * *", func() {
		log.Println("running scheduled cleanup task...")

		var retries int
		var cleanupSuccess bool

		// Retry logic
		for retries < maxRetries {
			log.Printf("attempt %d to clean up...", retries+1)
			err := CleanupAllExpired(24 * time.Hour, redisClient)  // Pass redisClient here
			if err == nil {
				log.Println("cleanup successful!")
				cleanupSuccess = true
				break
			} else {
				log.Printf("cleanup failed: %v", err)
				retries++
				time.Sleep(retryDelay) // Wait before retrying
			}
		}

		// If cleanup fails after retries, log the failure
		if !cleanupSuccess {
			log.Printf("cleanup task failed after %d retries. please check the system.", retries)

		// Call SendEmail to notify admin about the failure
			SendEmail(
			"admin@example.com", // Recipient email
		"The scheduled cleanup task failed after multiple attempts.", // Message body
		"Cleanup Task Failed", // Email subject
		"N/A", // OTP placeholder
		"",    // No attachment
		)
		}
	})

	// Start the cron scheduler
	c.Start()

	// Keep the main function running to let cron jobs execute
	select {}
}

func CleanBankPaymentDate(dateStr string) (string, error) {
	if dateStr == "" {
		return "", nil
	}
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", dateStr); err == nil {
		return dateStr, nil
	}
	return "", fmt.Errorf("invalid date format: %s", dateStr)
}
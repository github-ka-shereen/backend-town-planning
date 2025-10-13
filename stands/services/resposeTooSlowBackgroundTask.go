package services

import (
	"context"
	"fmt"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CheckIfProcessing checks if the task is currently being processed
func CheckIfProcessing(taskName string) (bool, error) {
	ctx := context.Background()
	rdb := config.InitRedisServer(ctx)

	processing, err := rdb.Get(ctx, taskName).Result()
	if err == redis.Nil {
		// Redis key does not exist, process the task
		return false, nil
	}
	if err != nil {
		// Redis error
		return false, fmt.Errorf("failed to check Redis status: %v", err)
	}
	// If the task is already in progress
	return processing == "true", nil
}

// StartBackgroundProcess starts a background process that performs a task and sends an email upon completion
func StartBackgroundProcess(
	taskName string,
	filters map[string]string,
	userEmail string,
	processFunc func(filters map[string]string) ([]interface{}, error),
	excelHeaders []string,
	subject string,
	message string,
) (*models.EmailLog, error) {
	// Initialize the Redis client using the config package
	ctx := context.Background()
	rdb := config.InitRedisServer(ctx)

	// Declare emailLog as a pointer to a models.EmailLog
	var emailLog *models.EmailLog

	// Set flag to true in Redis to indicate processing
	err := rdb.Set(ctx, taskName, "true", 0).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to set processing flag in Redis: %v", err)
	}

	// Offload the task to a background goroutine
	go func() {
		// Perform the long-running task using the provided process function
		allResults, err := processFunc(filters)
		if err != nil {
			// Log the error if any
			config.Logger.Error("Failed to process results in the background", zap.Error(err))
			return
		}

		// Once the task is done, reset the processing flag in Redis
		rdb.Set(ctx, taskName, "false", 0)

		// Generate the Excel file after processing is done
		filePath, err := GenerateExcel(allResults, taskName, excelHeaders)
		if err != nil {
			config.Logger.Error("Failed to generate Excel file", zap.Error(err))
			return
		}

		// Generate the download link for the generated Excel file
		downloadLink := GenerateDownloadLink(filePath)

		// Send email with the download link
		err = SendEmail(userEmail, message, subject, "", downloadLink)
		if err != nil {
			config.Logger.Error("Failed to send email", zap.Error(err))
			return
		}

		active := true

		// Create the email log after the background task completes
		emailLog = &models.EmailLog{
			ID:             uuid.New().String(),
			Recipient:      userEmail,
			Subject:        subject,
			Message:        message,
			SentAt:         Today(),
			Active:         &active,
			AttachmentPath: downloadLink,
		}

	}()

	// Umsebenzi find a way to log email
	// Return the emailLog (which will be populated after background task finishes)
	return emailLog, nil
}

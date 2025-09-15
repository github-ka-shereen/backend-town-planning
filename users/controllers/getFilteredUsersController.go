package controllers

import (
	"context"
	"fmt"
	"log"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/utils"
	"town-planning-backend/utils/pagination"

	"github.com/gofiber/fiber/v2"
)

func (uc *UserController) GetFilteredUsersController(c *fiber.Ctx) error {
	ctx := context.Background()
	rdb := config.InitRedisServer(ctx)

	// Parse and validate pagination params
	params := pagination.ParsePaginationParams(c)
	if err := pagination.ValidatePaginationParams(params); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid pagination parameters",
			"error":   err.Error(),
		})
	}

	// Generate the search and storage keys with resourceType as the prefix
	searchKey, storageKey := utils.GenerateHash("user", params.Filters, params.Page, params.PageSize)

	// Check if the file is already cached in Redis using the search key
	filePath, err := utils.FindMatchingFile(rdb, searchKey)
	if err == nil && filePath != "" {
		// File found in Redis, return it directly
		downloadURL := ""
		users, total, err := uc.UserRepo.GetFilteredUsers(
			params.Filters["start_date"],
			params.Filters["end_date"],
			params.PageSize,
			params.Page,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Error retrieving users",
				"error":   err.Error(),
			})
		}

		// Get the current number of results from the users slice
		currentNumberOfResults := len(users)

		// Check if total results exceed 5
		if pagination.CheckTotalResultsForDownload(currentNumberOfResults) {
			downloadURL = utils.GetDownloadURL(c, filePath)
		}

		// Create paginated response
		response := pagination.NewPaginatedResponse(c, users, total, params)

		return c.JSON(fiber.Map{
			"message": "Users retrieved successfully",
			"data": fiber.Map{
				"users":    response,
				"download": downloadURL,
			},
			"error": nil,
		})
	}

	// Implement Redis Locking to avoid generating file concurrently for the same query
	lockKey := fmt.Sprintf("lock:%s", searchKey)
	locked, err := rdb.SetNX(context.Background(), lockKey, "locked", 10*time.Second).Result()
	if err != nil {
		log.Printf("Error checking lock for key %s: %v", searchKey, err)
		return c.Status(500).JSON(fiber.Map{
			"message": "Error checking lock",
			"error":   err.Error(),
		})
	}

	if !locked {
		log.Printf("Another request is already generating the file for searchKey: %s", searchKey)
		return c.Status(429).JSON(fiber.Map{
			"message": "Another request is already generating the file",
		})
	}
	defer rdb.Del(context.Background(), lockKey) // Release the lock after the request completes

	// If file not found in Redis, generate it using the storage key
	users, total, err := uc.UserRepo.GetFilteredUsers(
		params.Filters["start_date"],
		params.Filters["end_date"],
		params.PageSize,
		params.Page,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Error retrieving users",
			"error":   err.Error(),
		})
	}

	// Get the current number of results from the users slice
	currentNumberOfResults := len(users)

	// Initialize downloadURL as an empty string
	var downloadURL string

	// Only generate Excel and URL if total results > 5
	if pagination.CheckTotalResultsForDownload(currentNumberOfResults) {
		headers := []string{"ID", "First Name", "Last Name", "Email", "Phone", "Role", "Created At"}

		// Generate the Excel file and get its file path
		filePath, err = utils.GenerateExcel(users, storageKey, headers)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"message": "Error generating Excel file",
				"error":   err.Error(),
			})
		}

		// Store the file in Redis with the storage key and the search key (use SETNX to avoid overwriting)
		err = rdb.SetNX(context.Background(), storageKey, filePath, 24*time.Hour).Err() // Cache for 24 hours
		if err != nil {
			log.Printf("Error caching file path: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"message": "Error caching file path",
				"error":   err.Error(),
			})
		}
		rdb.SetNX(context.Background(), searchKey, filePath, 24*time.Hour) // Cache for 24 hours

		// Generate the download URL
		downloadURL = utils.GetDownloadURL(c, filePath)
	}

	// Create paginated response
	response := pagination.NewPaginatedResponse(c, users, total, params)

	// Return response with or without the download URL based on total results
	if downloadURL != "" {
		return c.JSON(fiber.Map{
			"message": "Users retrieved successfully",
			"data": fiber.Map{
				"users":    response,
				"download": downloadURL,
			},
			"error": nil,
		})
	} else {
		return c.JSON(fiber.Map{
			"message": "Users retrieved successfully",
			"data": fiber.Map{
				"users":    response,
				"download": false, // Set download to false when no file is generated
			},
			"error": nil,
		})
	}
}

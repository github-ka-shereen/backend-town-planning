package controllers

import (
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// GetFilteredApplicationsController handles the fetching of filtered applications
func (ac *ApplicationController) GetFilteredApplicationsController(c *fiber.Ctx) error {
	// Parse query parameters
	pageSize := c.QueryInt("page_size", 10) // Default to 10 if not provided
	if pageSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid page_size parameter",
			"error":   "page_size must be greater than 0",
		})
	}

	page := c.QueryInt("page", 1) // Default to page 1 if not provided
	if page <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid page parameter",
			"error":   "page must be greater than 0",
		})
	}

	// Parse optional filters
	applicantID := c.Query("applicant_id")
	planNumber := c.Query("plan_number")
	permitNumber := c.Query("permit_number")
	status := c.Query("status")
	paymentStatus := c.Query("payment_status")
	standID := c.Query("stand_id")
	architectName := c.Query("architect_name")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	isCollected := c.Query("is_collected")

	// Calculate offset for pagination
	offset := (page - 1) * pageSize

	// Build filters map
	filters := make(map[string]string)
	if applicantID != "" {
		filters["applicant_id"] = applicantID
	}
	if planNumber != "" {
		filters["plan_number"] = planNumber
	}
	if permitNumber != "" {
		filters["permit_number"] = permitNumber
	}
	if status != "" {
		filters["status"] = status
	}
	if paymentStatus != "" {
		filters["payment_status"] = paymentStatus
	}
	if standID != "" {
		filters["stand_id"] = standID
	}
	if architectName != "" {
		filters["architect_name"] = architectName
	}
	if dateFrom != "" {
		filters["date_from"] = dateFrom
	}
	if dateTo != "" {
		filters["date_to"] = dateTo
	}
	if isCollected != "" {
		filters["is_collected"] = isCollected
	}

	// Fetch filtered applications from the repository
	applications, total, err := ac.ApplicationRepo.GetFilteredApplications(pageSize, offset, filters)
	if err != nil {
		config.Logger.Error("Failed to fetch filtered applications", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch applications",
			"error":   err.Error(),
		})
	}

	// Calculate total pages
	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	config.Logger.Info("Successfully fetched filtered applications",
		zap.Int("page", page),
		zap.Int("pageSize", pageSize),
		zap.Int64("total", total),
		zap.Int("resultsCount", len(applications)))

	// Return paginated response
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Applications fetched successfully",
		"data": fiber.Map{
			"data": applications,
			"meta": fiber.Map{
				"current_page": page,
				"page_size":    pageSize,
				"total":        total,
				"total_pages":  totalPages,
			},
		},
	})
}

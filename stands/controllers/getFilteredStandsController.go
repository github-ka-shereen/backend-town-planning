package controllers

import (
	"town-planning-backend/config"
	"town-planning-backend/stands/services"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

var DefaultPaymentFilters = map[string]bool{
	"status": true,
}

func (sc *StandController) GetFilteredStandsController(c *fiber.Ctx) error {
	// Parse query parameters
	pageSize := c.QueryInt("page_size", 5)
	if pageSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page_size parameter"})
	}

	page := c.QueryInt("page", 1)
	if page <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page parameter"})
	}

	// Clean up and sanitize the query parameters
	cleanQueryParam := func(param string) string {
		param = strings.TrimSpace(param)
		if param == "" || strings.ToLower(param) == "null" {
			return ""
		}
		return param
	}

	// Extract query parameters
	status := cleanQueryParam(c.Query("status"))
	projectID := cleanQueryParam(c.Query("project_id"))
	standCurrency := cleanQueryParam(c.Query("stand_currency"))
	startDate := cleanQueryParam(c.Query("start_date"))
	endDate := cleanQueryParam(c.Query("end_date"))
	userEmail := cleanQueryParam(c.Query("user_email"))

	// Validate user_email (optional)
	if userEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user_email parameter"})
	}

	// Calculate offset for pagination
	offset := (page - 1) * pageSize

	// Construct the filters map based on query parameters
	filters := make(map[string]string)
	if status != "" {
		filters["status"] = status
	}
	if standCurrency != "" {
		filters["stand_currency"] = standCurrency
	}
	if projectID != "" {
		filters["project_id"] = projectID
	}
	if startDate != "" && startDate != "null" {
		filters["start_date"] = startDate
	}
	if endDate != "" && endDate != "null" {
		filters["end_date"] = endDate
	}

	// Remove user_email from the filters as it should be handled separately
	delete(filters, "user_email")

	// Fetch paginated results based on filters
	paginatedPayments, total, err := sc.StandRepo.GetFilteredStands(filters, true, pageSize, offset)
	if err != nil {
		config.Logger.Error("Failed to fetch filtered stands", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch filtered stands"})
	}

	// config.Logger.Info("Total", zap.Int64("Total", total))

	// Calculate total pages for pagination
	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)
	response := fiber.Map{
		"data": paginatedPayments,
		"meta": fiber.Map{
			"current_page": page,
			"page_size":    pageSize,
			"total":        total,
			"total_pages":  totalPages,
		},
	}

	// Log filter values for debugging
	isDefault := services.IsDefaultStandsFilter(filters, DefaultPaymentFilters)
	// config.Logger.Info("filters", zap.Any("values", filters))
	config.Logger.Info("IsDefault", zap.Bool("Verdict", isDefault))
	// config.Logger.Info("UserEmail", zap.String("email", userEmail)) // Log the user email

	// Fetch all results if filters are non-default and pageSize > 1
	if !isDefault && pageSize > 1 {
		allResults, totalAll, isBackground, err := sc.StandRepo.GetFilteredAllStandsResults(filters, userEmail)
		if err != nil {
			config.Logger.Error("Failed to fetch all filtered stands", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch all filtered stands"})
		}

		// If task is in background, notify the user while sending current results
		if isBackground {
			response["message"] = "The operation is taking longer than expected and is now being processed in the background."
			return c.Status(fiber.StatusOK).JSON(response)
		}

		// Include all results in the response
		totalPages := (totalAll + int64(pageSize) - 1) / int64(pageSize)
		// Include all results in the response
		response["all_results"] = allResults
		response["all_results_meta"] = fiber.Map{"total": totalAll}
		response["all_results_meta"] = fiber.Map{"total_pages": totalPages}
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

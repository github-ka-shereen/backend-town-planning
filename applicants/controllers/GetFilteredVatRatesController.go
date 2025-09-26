package controllers

import (
	"strings"
	"town-planning-backend/config" // Assuming your models are here
	"town-planning-backend/utils"

	// Needed for time.Now() and time.Since()
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// DefaultFilters for VAT rates
var DefaultVatFilters = map[string]string{
	"active": "true",
}

// GetFilteredVatRatesController handles the API request for filtered VAT rates
func (pc *ApplicantController) GetFilteredVatRatesController(c *fiber.Ctx) error {
	// Parse query parameters
	pageSize := c.QueryInt("page_size", 5)
	if pageSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page_size parameter"})
	}

	page := c.QueryInt("page", 1)
	if page <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page parameter"})
	}

	// Helper function to clean query parameters
	cleanQueryParam := func(param string) string {
		param = strings.TrimSpace(param)
		if param == "" || strings.ToLower(param) == "null" {
			return ""
		}
		return param
	}

	is_active := cleanQueryParam(c.Query("is_active"))
	used := cleanQueryParam(c.Query("used"))
	startDate := cleanQueryParam(c.Query("start_date"))
	endDate := cleanQueryParam(c.Query("end_date"))
	userEmail := cleanQueryParam(c.Query("user_email"))

	// Validate user_email as it's mandatory for background processing
	if userEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user_email parameter"})
	}

	offset := (page - 1) * pageSize
	filters := make(map[string]string)
	if is_active != "" {
		filters["is_active"] = is_active
	}
	if used != "" {
		filters["used"] = used
	}
	if startDate != "" {
		filters["start_date"] = startDate
	}
	if endDate != "" {
		filters["end_date"] = endDate
	}

	// user_email is handled separately for background tasks, not for direct DB filtering
	delete(filters, "user_email")

	// Fetch paginated results
	paginatedRates, total, err := pc.ApplicantRepo.GetFilteredVatRates(pageSize, offset, filters)
	if err != nil {
		config.Logger.Error("Failed to fetch paginated VAT rates", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch VAT rates"})
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)
	response := fiber.Map{
		"data": paginatedRates,
		"meta": fiber.Map{
			"current_page": page,
			"page_size":    pageSize,
			"total":        total,
			"total_pages":  totalPages,
		},
	}

	isDefault := utils.IsDefaultFilter(filters, DefaultVatFilters)
	config.Logger.Info("VAT filters", zap.Any("values", filters))
	config.Logger.Info("VAT IsDefault", zap.Bool("Verdict", isDefault))
	config.Logger.Info("VAT UserEmail", zap.String("email", userEmail)) // Log the user email

	return c.Status(fiber.StatusOK).JSON(response)
}

package controllers

import (
	"strings"
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

var DefaultFilters = map[string]string{
	"active": "true",
}

func (uc *UserController) GetFilteredUsersController(c *fiber.Ctx) error {

	// Parse query parameters
	pageSize := c.QueryInt("page_size", 5)
	if pageSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page_size parameter"})
	}

	page := c.QueryInt("page", 1)
	if page <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page parameter"})
	}

	cleanQueryParam := func(param string) string {
		param = strings.TrimSpace(param)
		if param == "" || strings.ToLower(param) == "null" {
			return ""
		}
		return param
	}

	active := cleanQueryParam(c.Query("active"))
	startDate := cleanQueryParam(c.Query("start_date"))
	endDate := cleanQueryParam(c.Query("end_date"))
	userEmail := cleanQueryParam(c.Query("user_email"))

	// Validate user_email (optional)
	if userEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user_email parameter"})
	}

	offset := (page - 1) * pageSize
	filters := make(map[string]string)
	if active != "" {
		filters["active"] = active
	}
	if startDate != "" {
		filters["start_date"] = startDate
	}
	if endDate != "" {
		filters["end_date"] = endDate
	}

	delete(filters, "user_email")

	// Fetch paginated results
	paginatedRates, total, err := uc.UserRepo.GetFilteredUsers(pageSize, offset, filters)
	if err != nil {
		config.Logger.Error("Failed to fetch paginated users", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch users"})
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

	return c.Status(fiber.StatusOK).JSON(response)
}

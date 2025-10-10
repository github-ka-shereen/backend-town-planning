package controllers

import (
	"strings"
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (sc *StandController) GetFilteredStandTypesController(c *fiber.Ctx) error {
	// Parse query parameters
	pageSize := c.QueryInt("page_size", 10)
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

	// Get query parameters
	active := cleanQueryParam(c.Query("active"))
	startDate := cleanQueryParam(c.Query("start_date"))
	endDate := cleanQueryParam(c.Query("end_date"))
	name := cleanQueryParam(c.Query("name"))
	createdBy := cleanQueryParam(c.Query("created_by"))
	isSystem := cleanQueryParam(c.Query("is_system"))

	offset := (page - 1) * pageSize
	filters := make(map[string]string)

	// Build filters
	if active != "" && active != "undefined" {
		filters["active"] = active
	}
	if startDate != "" && startDate != "null" {
		filters["start_date"] = startDate
	}
	if endDate != "" && endDate != "null" {
		filters["end_date"] = endDate
	}
	if name != "" && name != "undefined" {
		filters["name"] = name
	}
	if createdBy != "" && createdBy != "undefined" {
		filters["created_by"] = createdBy
	}
	if isSystem != "" && isSystem != "undefined" {
		filters["is_system"] = isSystem
	}

	// Fetch paginated results
	paginatedStandTypes, total, err := sc.StandRepo.GetFilteredStandTypes(pageSize, offset, filters)
	if err != nil {
		config.Logger.Error("Failed to fetch paginated stand types", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch stand types"})
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)
	response := fiber.Map{
		"data": paginatedStandTypes,
		"meta": fiber.Map{
			"current_page": page,
			"page_size":    pageSize,
			"total":        total,
			"total_pages":  totalPages,
		},
	}

	return c.Status(fiber.StatusOK).JSON(response)
}
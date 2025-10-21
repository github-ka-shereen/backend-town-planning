package controllers

import (
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// GetFilteredApprovalGroupsController handles the fetching of filtered approval groups
func (ac *ApplicationController) GetFilteredApprovalGroupsController(c *fiber.Ctx) error {
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
	name := c.Query("name")
	groupType := c.Query("type")
	isActive := c.Query("is_active")
	requiresAllApprovals := c.Query("requires_all_approvals")
	autoAssignBackups := c.Query("auto_assign_backups")
	hasActiveMembers := c.Query("has_active_members")
	createdBy := c.Query("created_by")

	// Calculate offset for pagination
	offset := (page - 1) * pageSize

	// Build filters map
	filters := make(map[string]string)
	if name != "" {
		filters["name"] = name
	}
	if groupType != "" {
		filters["type"] = groupType
	}
	if isActive != "" {
		filters["is_active"] = isActive
	}
	if requiresAllApprovals != "" {
		filters["requires_all_approvals"] = requiresAllApprovals
	}
	if autoAssignBackups != "" {
		filters["auto_assign_backups"] = autoAssignBackups
	}
	if hasActiveMembers != "" {
		filters["has_active_members"] = hasActiveMembers
	}
	if createdBy != "" {
		filters["created_by"] = createdBy
	}

	// Fetch filtered approval groups from the repository
	approvalGroups, total, err := ac.ApplicationRepo.GetFilteredApprovalGroups(pageSize, offset, filters)
	if err != nil {
		config.Logger.Error("Failed to fetch filtered approval groups", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch approval groups",
			"error":   err.Error(),
		})
	}

	// Calculate total pages
	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	config.Logger.Info("Successfully fetched filtered approval groups",
		zap.Int("page", page),
		zap.Int("pageSize", pageSize),
		zap.Int64("total", total),
		zap.Int("resultsCount", len(approvalGroups)))

	// Return paginated response
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Approval groups fetched successfully",
		"data": fiber.Map{
			"data": approvalGroups,
			"meta": fiber.Map{
				"current_page": page,
				"page_size":    pageSize,
				"total":        total,
				"total_pages":  totalPages,
			},
		},
	})
}
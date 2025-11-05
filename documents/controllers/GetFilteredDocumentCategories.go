package controllers

// import (
// 	"town-planning-backend/config"

// 	"github.com/gofiber/fiber/v2"
// 	"go.uber.org/zap"
// )

// func (dc *DocumentController) FilteredDocumentCategories(c *fiber.Ctx) error {
// 	// Parse query parameters
// 	pageSize := c.QueryInt("page_size", 5) // Default to 10 if not provided
// 	if pageSize <= 0 {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid page_size parameter",
// 		})
// 	}

// 	page := c.QueryInt("page", 1) // Default to page 1 if not provided
// 	if page <= 0 {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid page parameter",
// 		})
// 	}

// 	// Calculate offset for pagination
// 	offset := (page - 1) * pageSize

// 	// Fetch filtered documents from the repository
// 	filteredDocuments, total, err := dc.DocumentRepo.GetFilteredDocumentCategories(pageSize, offset)
// 	if err != nil {
// 		config.Logger.Error("Failed to fetch filtered documents", zap.Error(err))
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to fetch documents",
// 		})
// 	}

// 	// Convert total to int (or pageSize to int64) before the division
// 	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

// 	// Return paginated response
// 	return c.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"data": filteredDocuments,
// 		"meta": fiber.Map{
// 			"current_page": page,
// 			"page_size":    pageSize,
// 			"total":        total,
// 			"total_pages":  totalPages,
// 		},
// 	})
// }

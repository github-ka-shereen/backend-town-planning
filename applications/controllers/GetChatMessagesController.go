// controllers/chat_controller.go
package controllers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// controllers/chat_controller.go
func (cc *ApplicationController) GetChatMessagesController(c *fiber.Ctx) error {
	threadID := c.Params("threadId")
	if threadID == "" {
		return c.Status(400).JSON(fiber.Map{
			"message": "Thread ID is required",
			"error":   "missing_thread_id",
		})
	}

	// Get pagination parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	// Use repository method
	messages, total, err := cc.ApplicationRepo.GetChatMessagesWithPreload(threadID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Calculate pagination
	totalInt := int(total)
	totalPages := (totalInt + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"messages": messages,
			"pagination": fiber.Map{
				"page":       page,
				"limit":      limit,
				"total":      totalInt,
				"totalPages": totalPages,
				"hasNext":    page < totalPages,
				"hasPrev":    page > 1,
			},
		},
		"message": "Chat messages retrieved successfully",
	})
}

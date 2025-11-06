// controllers/chat_controller.go
package controllers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (cc *ApplicationController) GetChatMessagesController(c *fiber.Ctx) error {
	// FIXED: Use the correct parameter name that matches your route
	threadID := c.Params("threadId") // Make sure this matches your route definition
	if threadID == "" {
		return c.Status(400).JSON(fiber.Map{
			"message": "Thread ID is required",
			"error":   "missing_thread_id",
		})
	}

	// Get pagination parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Fetch messages from repository
	messages, total, err := cc.ApplicationRepo.GetChatMessagesWithPreload(threadID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Failed to fetch chat messages",
			"error":   err.Error(),
		})
	}

	// Calculate pagination info
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// Return response
	return c.JSON(fiber.Map{
		"message": "Chat messages retrieved successfully",
		"data": fiber.Map{
			"messages": messages,
			"pagination": fiber.Map{
				"page":       page,
				"limit":      limit,
				"total":      total,
				"totalPages": totalPages,
				"hasNext":    page < totalPages,
				"hasPrev":    page > 1,
			},
		},
		"error": nil,
	})
}

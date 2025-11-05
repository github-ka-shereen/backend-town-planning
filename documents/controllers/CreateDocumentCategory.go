package controllers

// import (
// 	"town-planning-backend/db/models"
// 	"net/http"

// 	"time"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/google/uuid"
// )

// type CreateDocumentCategoryRequest struct {
// 	CategoryName string `json:"category_name"`
// 	Description  string `json:"description"`
// 	CreatedBy    string `json:"created_by"`
// }

// func (dc *DocumentController) CreateDocumentCategory(c *fiber.Ctx) error {
// 	request := new(CreateDocumentCategoryRequest)

// 	if err := c.BodyParser(request); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request body",
// 		})
// 	}

// 	// Create a new DocumentCategory model from the request
// 	category := &models.DocumentCategory{
// 		ID:           uuid.New(),
// 		CategoryName: request.CategoryName,
// 		Description:  request.Description,
// 		CreatedBy:    request.CreatedBy,
// 		CreatedAt:    time.Now(),
// 		UpdatedAt:    time.Now(),
// 	}

// 	createdCategory, err := dc.DocumentRepo.CreateCategory(category)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to create document category",
// 		})
// 	}

// 	// Return the created category as JSON
// 	return c.Status(http.StatusCreated).JSON(createdCategory)
// }

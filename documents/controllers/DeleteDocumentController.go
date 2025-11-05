package controllers

import (
	"net/http"
	"town-planning-backend/config"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DeleteDocumentRequest struct {
	ID uuid.UUID `json:"id" binding:"required,uuid"`
}

func (dc *DocumentController) DeleteDocument(c *fiber.Ctx) error {
	config.Logger.Info("Delete document request received")

	// 1. Parse the document ID from the request parameters
	idParam := c.Params("id")
	documentID, err := uuid.Parse(idParam)
	if err != nil {
		config.Logger.Error("Invalid document ID format", zap.String("id", idParam), zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid document ID format"})
	}

	// 2. Call the repository function to soft delete the document
	err = dc.DocumentRepo.DeleteDocument(documentID)
	if err != nil {
		config.Logger.Error("Failed to soft delete document", zap.String("document_id", documentID.String()), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete document"})
	}

	// 3. Respond with a success message
	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "Document deleted successfully"})
}

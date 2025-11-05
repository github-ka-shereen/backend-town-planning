package router

import (
	applicants_repositories "town-planning-backend/applicants/repositories"
	document_controllers "town-planning-backend/documents/controllers"
	document_repositories "town-planning-backend/documents/repositories"
	"town-planning-backend/documents/services"
	internal_services "town-planning-backend/internal/services"
	stand_repositories "town-planning-backend/stands/repositories"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func DocumentRouterInit(app *fiber.App,
	db *gorm.DB,
	standRepository stand_repositories.StandRepository,
	applicantRepository applicants_repositories.ApplicantRepository,
	documentRepository document_repositories.DocumentRepository,
	geminiService *internal_services.GeminiService,
	documentService *services.DocumentService,
) {
	documentController := &document_controllers.DocumentController{
		DB:              db,
		StandRepo:       standRepository,
		ApplicantRepo:   applicantRepository,
		DocumentRepo:    documentRepository,
		GeminiService:   geminiService,
		DocumentService: documentService,
	}

	// app.Post("/api/v1/documents/categories", documentController.CreateDocumentCategory)
	app.Post("/api/v1/documents", documentController.CreateDocument)
	// app.Get("/api/v1/filtered/document-categories", documentController.FilteredDocumentCategories)
	app.Get("/api/v1/documents-payment-plans/:id", documentController.GetDocumentsByPlanID)
	app.Delete("/api/v1/documents/:id", documentController.DeleteDocument)
}

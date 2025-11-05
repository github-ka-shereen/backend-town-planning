package routes

import (
	controllers "town-planning-backend/applications/controllers"
	repositories "town-planning-backend/applications/repositories"
	indexing_repository "town-planning-backend/bleve/repositories"
	user_repository "town-planning-backend/users/repositories"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ApplicationRouterInit(
	app *fiber.App,
	db *gorm.DB,
	applicationRepository repositories.ApplicationRepository,
	bleveRepository indexing_repository.BleveRepositoryInterface,
	userRepo user_repository.UserRepository,
) {
	applicationController := &controllers.ApplicationController{
		ApplicationRepo: applicationRepository,
		DB:              db,
		BleveRepo:       bleveRepository,
		UserRepo:        userRepo,
	}

	applicationRoutes := app.Group("/api/v1")

	// Development Categories
	applicationRoutes.Post("/development-categories", applicationController.CreateDevelopmentCategory)
	applicationRoutes.Get("/development-categories", applicationController.GetAllDevelopmentCategories)
	applicationRoutes.Get("/all-development-categories", applicationController.GetAllActiveDevelopmentCategories)

	// Tariffs
	applicationRoutes.Post("/add-new-tariff", applicationController.CreateNewTariff)
	applicationRoutes.Get("/filtered-development-tariffs", applicationController.GetFilteredDevelopmentTariffsController)

	// Approval Groups
	applicationRoutes.Post("/approval-groups/create-with-members", applicationController.CreateApprovalGroupWithMembers)
	applicationRoutes.Get("/filtered-approval-groups", applicationController.GetFilteredApprovalGroupsController)

	// Applications - Comprehensive endpoints
	applicationRoutes.Post("/create-application", applicationController.CreateApplicationController)
	applicationRoutes.Get("/filtered-applications", applicationController.GetFilteredApplicationsController)
	applicationRoutes.Get("/application/:id", applicationController.GetApplicationByIdController)


	// New comprehensive update endpoint - updates ALL fields
	applicationRoutes.Put("/applications/:id", applicationController.UpdateApplicationDetailsController)

	// New granular update endpoints
	applicationRoutes.Patch("/applications/:id/status", applicationController.UpdateApplicationStatusController)
	applicationRoutes.Patch("/applications/:id/architect", applicationController.UpdateApplicationArchitectController)
	applicationRoutes.Patch("/applications/:id/costs", applicationController.RecalculateApplicationCostsController)
	applicationRoutes.Patch("/applications/:id/collection", applicationController.MarkApplicationCollectedController)
	applicationRoutes.Patch("/applications/:id/document-flags", applicationController.UpdateDocumentFlagsController)

	// Application Documents
	// applicationRoutes.Get("/applications/:id/documents", applicationController.GetApplicationDocumentsController)
	// applicationRoutes.Post("/applications/:id/documents", applicationController.UploadApplicationDocumentController)
	// applicationRoutes.Delete("/applications/:id/documents/:documentId", applicationController.DeleteApplicationDocumentController)

	// Application Actions (MUST come before generic :id routes)
	applicationRoutes.Post("/generate-tpd-1-form/:id", applicationController.GenerateTPD1FormController)
	applicationRoutes.Get("/application-approval-data/:id", applicationController.GetApplicationApprovalDataController)

	// Approval Workflow - Use POST for actions that change state
	applicationRoutes.Post("/applications/:id/approve", applicationController.ApproveRejectApplicationController)
	applicationRoutes.Post("/applications/:id/reject", applicationController.RejectApplicationController)
	applicationRoutes.Post("/applications/:id/issues", applicationController.RaiseIssueController)

	// New approval workflow endpoints
	// applicationRoutes.Post("/applications/:id/assign-group", applicationController.AssignApplicationToGroupController)
}

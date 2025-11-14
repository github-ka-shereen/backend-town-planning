package routes

import (
	applicants_repositories "town-planning-backend/applicants/repositories"
	controllers "town-planning-backend/applications/controllers"
	repositories "town-planning-backend/applications/repositories"
	indexing_repository "town-planning-backend/bleve/repositories"
	documents_services "town-planning-backend/documents/services"
	user_repository "town-planning-backend/users/repositories"
	"town-planning-backend/websocket"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ApplicationRouterInit(
	app *fiber.App,
	db *gorm.DB,
	applicationRepository repositories.ApplicationRepository,
	bleveRepository indexing_repository.BleveRepositoryInterface,
	userRepo user_repository.UserRepository,
	documentService *documents_services.DocumentService,
	applicantRepo applicants_repositories.ApplicantRepository,
	wsHub *websocket.Hub, // Added WebSocket hub for real-time features
) {
	applicationController := &controllers.ApplicationController{
		ApplicationRepo: applicationRepository,
		DB:              db,
		BleveRepo:       bleveRepository,
		UserRepo:        userRepo,
		DocumentSvc:     documentService,
		ApplicantRepo:   applicantRepo,
		WsHub:           wsHub, // Added WebSocket hub to controller
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
	applicationRoutes.Post("/applications/:id/process-application-submission", applicationController.ProcessApplicationSubmissionController)

	// New granular update endpoints
	applicationRoutes.Patch("/applications/:id/status", applicationController.UpdateApplicationStatusController)
	applicationRoutes.Patch("/applications/:id/architect", applicationController.UpdateApplicationArchitectController)
	applicationRoutes.Patch("/applications/:id/costs", applicationController.RecalculateApplicationCostsController)
	applicationRoutes.Patch("/applications/:id/collection", applicationController.MarkApplicationCollectedController)
	applicationRoutes.Patch("/applications/:id/document-flags", applicationController.UpdateDocumentFlagsController)

	// Application Actions (MUST come before generic :id routes)
	applicationRoutes.Post("/generate-tpd-1-form/:id", applicationController.GenerateTPD1FormController)
	applicationRoutes.Get("/application-approval-data/:id", applicationController.GetApplicationApprovalDataController)

	// Generate Comments Sheet
	applicationRoutes.Post("/generate-comments-sheet/:id", applicationController.GenerateCommentsSheetController)

	// Generate Development Permit
	applicationRoutes.Post("/generate-development-permit/:id", applicationController.GenerateDevelopmentPermitController)

	// Chat Messages - ADDED THIS ROUTE
	applicationRoutes.Get("/chat/threads/:threadId/messages", applicationController.GetChatMessagesController)

	// Approval Workflow - Use POST for actions that change state
	applicationRoutes.Post("/applications/:id/approve", applicationController.ApproveRejectApplicationController)
	applicationRoutes.Post("/applications/:id/reject", applicationController.RejectApplicationController)
	
	// ADD REVOKE ENDPOINT HERE
	applicationRoutes.Post("/applications/:id/revoke", applicationController.RevokeDecisionController)
	
	applicationRoutes.Post("/applications/:id/raise-issue", applicationController.RaiseIssueController)
	applicationRoutes.Post("/issues/:id/resolve", applicationController.ResolveIssueController)
	applicationRoutes.Post("/issues/:id/reopen", applicationController.ReopenIssueController)
	applicationRoutes.Post("/chat/threads/:threadId/messages", applicationController.SendMessageController)

	// Real-time Chat Features - ADDED THESE ROUTES
	applicationRoutes.Post("/chat/threads/:threadId/typing", applicationController.HandleTypingIndicator) // Typing indicators
	applicationRoutes.Post("/chat/threads/:threadId/read", applicationController.MarkMessagesAsRead)      // Read receipts
	applicationRoutes.Get("/chat/threads/:threadId/unread", applicationController.GetUnreadCount)         // Unread message count

	// Unified Chat Participants Management (SINGLE ENDPOINT)
	applicationRoutes.Post("/chat/threads/:threadId/participants", applicationController.UnifiedParticipantController)

	// Get Thread Participants (Separate GET endpoint)
	applicationRoutes.Get("/chat/threads/:threadId/participants", applicationController.GetThreadParticipantsController)

	// New approval workflow endpoints
	// applicationRoutes.Post("/applications/:id/assign-group", applicationController.AssignApplicationToGroupController)

	// Chat Message Features - ADD THESE ROUTES
	applicationRoutes.Post("/chat/messages/:messageId/star", applicationController.StarMessageController)
	applicationRoutes.Post("/chat/messages/:messageId/reply", applicationController.ReplyToMessageController)
	applicationRoutes.Delete("/chat/messages/:messageId", applicationController.DeleteMessageController)
	applicationRoutes.Get("/chat/messages/:messageId/stars", applicationController.GetMessageStarsController)
	applicationRoutes.Get("/chat/messages/:messageId/thread", applicationController.GetMessageThreadController)
}
package routes

import (
	controllers "town-planning-backend/applications/controllers"
	repositories "town-planning-backend/applications/repositories"
	indexing_repository "town-planning-backend/bleve/repositories"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ApplicationRouterInit(
	app *fiber.App,
	db *gorm.DB,
	applicationRepository repositories.ApplicationRepository,
	bleveRepository indexing_repository.BleveRepositoryInterface,
) {
	applicationController := &controllers.ApplicationController{
		ApplicationRepo: applicationRepository,
		DB:              db,
		BleveRepo:       bleveRepository,
	}

	applicationRoutes := app.Group("/api/v1")
	applicationRoutes.Post("/development-categories", applicationController.CreateDevelopmentCategory)
	applicationRoutes.Get("/development-categories", applicationController.GetAllDevelopmentCategories)
	applicationRoutes.Get("/all-development-categories", applicationController.GetAllActiveDevelopmentCategories)
	applicationRoutes.Post("/add-new-tariff", applicationController.CreateNewTariff)
	applicationRoutes.Get("/filtered-development-tariffs", applicationController.GetFilteredDevelopmentTariffsController)
	applicationRoutes.Post("/create-application", applicationController.CreateApplicationController)
	applicationRoutes.Get("/filtered-applications", applicationController.GetFilteredApplicationsController)
	applicationRoutes.Get("/application/:id", applicationController.GetApplicationByIdController)
}

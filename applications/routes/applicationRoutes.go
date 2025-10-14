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
}

package routes

import (
	"town-planning-backend/stands/controllers"
	"town-planning-backend/stands/repositories"
	indexing_repository "town-planning-backend/bleve/repositories"	

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func StandRouterInit(
	app *fiber.App,
	db *gorm.DB,
	standRepository repositories.StandRepository,
    bleveRepository indexing_repository.BleveRepositoryInterface, 
) {
	standController := &controllers.StandController{
		StandRepo: standRepository,
		DB:        db,
		BleveRepo: bleveRepository,
	}

	standRoutes := app.Group("/api/v1/stands")
	standRoutes.Post("/stand-types", standController.AddStandTypesController)
	standRoutes.Post("/bulk-upload-projects", standController.BulkUploadProjects)
	standRoutes.Post("/create-project", standController.CreateProject)
	standRoutes.Get("/stand-types/filtered", standController.GetFilteredStandTypesController)
	standRoutes.Get("/projects/filtered", standController.GetFilteredProjectsController)
}

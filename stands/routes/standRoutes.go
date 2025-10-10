package routes

import (
	"town-planning-backend/stands/controllers"
	"town-planning-backend/stands/repositories"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func StandRouterInit(
	app *fiber.App,
	db *gorm.DB,
	standRepository repositories.StandRepository,
) {
	standController := &controllers.StandController{
		StandRepo: standRepository,
		DB:        db,
	}

	standRoutes := app.Group("/stands")
	standRoutes.Post("/stand-types", standController.AddStandTypesController)
}

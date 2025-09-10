package routes

import (
	"town-planning-backend/bleve/controllers"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func InitBleveRoutes(app *fiber.App, controller *controllers.SearchController, db *gorm.DB) {
	api := app.Group("/api/v1/bleve_search")

	api.Get("/users", controller.SearchUsersController)

}

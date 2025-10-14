package routes

import (
	controllers "town-planning-backend/applicants/controllers"
	"town-planning-backend/applicants/repositories"
	indexing_repository "town-planning-backend/bleve/repositories"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ApplicantInitRoutes(
	app *fiber.App,
	applicantRepo repositories.ApplicantRepository,
	bleveInterfaceRepo indexing_repository.BleveRepositoryInterface,
	db *gorm.DB,
) {
	applicantController := &controllers.ApplicantController{
		ApplicantRepo: applicantRepo,
		DB:            db,
		BleveRepo:     bleveInterfaceRepo,
	}

	// Create API v1 group
	api := app.Group("/api/v1")

	api.Post("/applicants", applicantController.CreateApplicantController)
	api.Get("/applicants/filtered", applicantController.GetFilteredApplicantsController)
	api.Post("/applicants/vat-rates", applicantController.CreateVATRateController)
	api.Get("/applicants/vat-rates/filtered", applicantController.GetFilteredVatRatesController)
	api.Get("/applicants/vat-rates/active", applicantController.GetActiveVATRateController)
}
package controllers

import (
	indexing_repository "town-planning-backend/bleve/repositories"
	"town-planning-backend/applications/repositories"
	applicant_repository "town-planning-backend/applicants/repositories"
	user_repository "town-planning-backend/users/repositories"
	documents_services "town-planning-backend/documents/services"

	"gorm.io/gorm"
)

type ApplicationController struct {
	ApplicationRepo repositories.ApplicationRepository
	ApplicantRepo   applicant_repository.ApplicantRepository
	DB              *gorm.DB
	BleveRepo       indexing_repository.BleveRepositoryInterface
	UserRepo        user_repository.UserRepository
	DocumentSvc     *documents_services.DocumentService
}

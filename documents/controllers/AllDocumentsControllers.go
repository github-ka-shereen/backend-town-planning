package controllers

import (
	applicant_repo "town-planning-backend/applicants/repositories"
	document_repositories "town-planning-backend/documents/repositories"
	"town-planning-backend/documents/services"
	internal_services "town-planning-backend/internal/services"
	stand_repos "town-planning-backend/stands/repositories"

	"gorm.io/gorm"
)

type DocumentController struct {
	DB              *gorm.DB
	StandRepo       stand_repos.StandRepository
	ApplicantRepo   applicant_repo.ApplicantRepository
	DocumentRepo    document_repositories.DocumentRepository
	GeminiService   *internal_services.GeminiService
	DocumentService *services.DocumentService
}

func NewDocumentController(
	db *gorm.DB,
	standRepo stand_repos.StandRepository,
	applicantRepo applicant_repo.ApplicantRepository,
	documentRepo document_repositories.DocumentRepository,
	geminiService *internal_services.GeminiService,
	documentService *services.DocumentService,
) *DocumentController {
	return &DocumentController{
		DB:              db,
		StandRepo:       standRepo,
		ApplicantRepo:   applicantRepo,
		DocumentRepo:    documentRepo,
		GeminiService:   geminiService,
		DocumentService: documentService,
	}
}

package controllers

import (
	applicant_repository "town-planning-backend/applicants/repositories"
	"town-planning-backend/applications/repositories"
	indexing_repository "town-planning-backend/bleve/repositories"
	documents_services "town-planning-backend/documents/services"
	user_repository "town-planning-backend/users/repositories"
	websocket "town-planning-backend/websocket"

	"gorm.io/gorm"
)

type ApplicationController struct {
	ApplicationRepo repositories.ApplicationRepository
	ApplicantRepo   applicant_repository.ApplicantRepository
	DB              *gorm.DB
	BleveRepo       indexing_repository.BleveRepositoryInterface
	UserRepo        user_repository.UserRepository
	DocumentSvc     *documents_services.DocumentService
	WsHub           *websocket.Hub // Added WebSocket hub for real-time features
}

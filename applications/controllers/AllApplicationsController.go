package controllers

import (
	indexing_repository "town-planning-backend/bleve/repositories"
	"town-planning-backend/applications/repositories"

	"gorm.io/gorm"
)

type ApplicationController struct {
	ApplicationRepo repositories.ApplicationRepository
	DB              *gorm.DB
	BleveRepo       indexing_repository.BleveRepositoryInterface
}

package controllers

import (
	indexing_repository "town-planning-backend/bleve/repositories"
	"town-planning-backend/applications/repositories"
	user_repository "town-planning-backend/users/repositories"

	"gorm.io/gorm"
)

type ApplicationController struct {
	ApplicationRepo repositories.ApplicationRepository
	DB              *gorm.DB
	BleveRepo       indexing_repository.BleveRepositoryInterface
	UserRepo        user_repository.UserRepository
}

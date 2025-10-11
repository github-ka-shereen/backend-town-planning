package controllers

import (
	indexing_repository "town-planning-backend/bleve/repositories"
	"town-planning-backend/stands/repositories"

	"gorm.io/gorm"
)

type StandController struct {
	StandRepo repositories.StandRepository
	DB        *gorm.DB
	BleveRepo indexing_repository.BleveRepositoryInterface
}

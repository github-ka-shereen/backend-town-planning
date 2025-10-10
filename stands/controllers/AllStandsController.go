package controllers

import (
	"town-planning-backend/stands/repositories"

	"gorm.io/gorm"
)

type StandController struct {
	StandRepo repositories.StandRepository
	DB        *gorm.DB
}

package repositories

import (
	"town-planning-backend/db/models"

	"gorm.io/gorm"
)

type StandRepository interface {
	AddStandTypes(tx *gorm.DB, standType *models.StandType) (*models.StandType, error)
}

type standRepository struct {
	db *gorm.DB
}

func NewStandRepository(db *gorm.DB) StandRepository {
	return &standRepository{
		db: db,
	}
}

// AddStandTypes creates a new stand type in the database
func (r *standRepository) AddStandTypes(tx *gorm.DB, standType *models.StandType) (*models.StandType, error) {
	if err := tx.Create(standType).Error; err != nil {
		return nil, err
	}
	return standType, nil
}

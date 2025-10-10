package repositories

import "gorm.io/gorm"

type StandRepository interface {
}

type standRepository struct {
	db *gorm.DB
}

func NewStandRepository(db *gorm.DB) StandRepository {
	return &standRepository{
		db: db,
	}
}

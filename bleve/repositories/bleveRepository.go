package repositories

import (
	"context"
	bleveindex "town-planning-backend/bleve/services"
	"town-planning-backend/db/models"
)

type BleveRepository struct {
	indexer *bleveindex.IndexingService
}

type BleveRepositoryInterface interface {
	// General
	DeleteAllIndices(ctx context.Context) error

	// ==== User Indexing ====
	IndexSingleUser(user models.User) error
	IndexExistingUsers(users []models.User) error
	UpdateUser(user models.User) error
	DeleteUser(userID string) error
}

// Constructor returning both the struct and the interface
func NewBleveRepository(indexer *bleveindex.IndexingService) (*BleveRepository, BleveRepositoryInterface) {
	repo := &BleveRepository{indexer: indexer}
	return repo, repo
}

func (r *BleveRepository) DeleteAllIndices(ctx context.Context) error {
	return r.indexer.DeleteAllIndices()
}

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

	// ==== Applicant Indexing ====
	IndexSingleApplicant(applicant models.Applicant) error
	IndexExistingApplicants(applicants []models.Applicant) error
	UpdateApplicant(applicant models.Applicant) error
	DeleteApplicant(applicantID string) error

	// ==== VAT Rate Indexing ====
	IndexSingleVATRate(vatRate models.VATRate) error
	IndexExistingVATRates(vatRates []models.VATRate) error
	UpdateVATRate(vatRate models.VATRate) error
	DeleteVATRate(vatRateID string) error

	// ==== Project Indexing ====
	IndexSingleProject(project models.Project) error
	IndexExistingProjects(projects []models.Project) error
	UpdateProject(project models.Project) error
	DeleteProject(projectID string) error

	// ==== Stand Indexing ====
	IndexSingleStand(stand models.Stand) error
	IndexExistingStands(stands []models.Stand) error
	UpdateStand(stand models.Stand) error
	DeleteStand(standID string) error
}

// Constructor returning both the struct and the interface
func NewBleveRepository(indexer *bleveindex.IndexingService) (*BleveRepository, BleveRepositoryInterface) {
	repo := &BleveRepository{indexer: indexer}
	return repo, repo
}

func (r *BleveRepository) DeleteAllIndices(ctx context.Context) error {
	return r.indexer.DeleteAllIndices()
}

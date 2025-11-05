package repositories

import (
	"errors"
	"fmt"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	stand_repositories "town-planning-backend/stands/repositories"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DocumentRepository interface {
	GetDocumentsByPlanID(planUUID string) ([]models.Document, error)
	CreateDocument(tx *gorm.DB, document *models.Document) (*models.Document, error)
	CreateDocumentWithAudit(tx *gorm.DB, document *models.Document, userID, userName, userRole, ipAddress, userAgent string) (*models.Document, error)
	DeleteDocument(id uuid.UUID) error
	UpdateDocument(tx *gorm.DB, documentID uuid.UUID, updates map[string]interface{}) ([]models.Document, error)
	GetCategoryByCode(tx *gorm.DB, code string) (*models.DocumentCategory, error)
	CreateCategory(tx *gorm.DB, category *models.DocumentCategory) (*models.DocumentCategory, error)
	GetApplicant(tx *gorm.DB, applicantID uuid.UUID) (*models.Applicant, error)
	FindExistingDocument(tx *gorm.DB, categoryID uuid.UUID, entityType string, entityID *uuid.UUID) (*models.Document, error)

	// Methods for normalized model
	GetDocumentsByEntity(tx *gorm.DB, entityType string, entityID uuid.UUID) ([]models.Document, error)
	CreateEntityDocumentRelationship(tx *gorm.DB, relationship interface{}) error
	DeleteEntityDocumentRelationships(tx *gorm.DB, documentID uuid.UUID) error
	GetDocumentWithRelationships(tx *gorm.DB, documentID uuid.UUID) (*models.Document, error)
}

type documentRepository struct {
	StandRepo stand_repositories.StandRepository
	db        *gorm.DB
}

func NewDocumentRepository(db *gorm.DB, standRepo stand_repositories.StandRepository) DocumentRepository {
	return &documentRepository{
		db:        db,
		StandRepo: standRepo,
	}
}

func (r *documentRepository) CreateDocument(tx *gorm.DB, document *models.Document) (*models.Document, error) {
	if err := tx.Create(document).Error; err != nil {
		return nil, err
	}
	return document, nil
}

func (r *documentRepository) CreateDocumentWithAudit(
	tx *gorm.DB,
	document *models.Document,
	userID, userName, userRole, ipAddress, userAgent string,
) (*models.Document, error) {

	if err := tx.Create(document).Error; err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	// Create audit log entry
	auditLog := &models.DocumentAuditLog{
		ID:         uuid.New(),
		DocumentID: document.ID,
		Action:     models.ActionCreate,
		UserID:     userID,
		UserName:   &userName,
		UserRole:   &userRole,
		Reason:     nil,
		IPAddress:  &ipAddress,
		UserAgent:  &userAgent,

		// Document state after creation
		NewFileName:    &document.FileName,
		NewCategoryID:  document.CategoryID,
		NewDescription: document.Description,
		NewIsPublic:    &document.IsPublic,
		NewIsMandatory: &document.IsMandatory,
		NewIsActive:    &document.IsActive,

		CreatedAt: time.Now(),
	}

	if err := tx.Create(auditLog).Error; err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	return document, nil
}

func (r *documentRepository) GetCategoryByCode(tx *gorm.DB, code string) (*models.DocumentCategory, error) {
	var category models.DocumentCategory
	err := tx.Where("code = ? AND is_active = ?", code, true).First(&category).Error
	if err != nil {
		return nil, fmt.Errorf("category not found with code '%s': %w", code, err)
	}
	return &category, nil
}

func (r *documentRepository) CreateCategory(tx *gorm.DB, category *models.DocumentCategory) (*models.DocumentCategory, error) {
	if err := tx.Create(category).Error; err != nil {
		return nil, fmt.Errorf("failed to create document category: %w", err)
	}
	return category, nil
}

func (r *documentRepository) GetApplicant(tx *gorm.DB, applicantID uuid.UUID) (*models.Applicant, error) {
	var applicant models.Applicant
	err := tx.First(&applicant, "id = ?", applicantID).Error
	if err != nil {
		return nil, fmt.Errorf("applicant not found: %w", err)
	}
	return &applicant, nil
}

func (r *documentRepository) FindExistingDocument(tx *gorm.DB, categoryID uuid.UUID, entityType string, entityID *uuid.UUID) (*models.Document, error) {
	config.Logger.Info("🔍 FindExistingDocument query starting",
		zap.String("entityType", entityType),
		zap.Any("entityID", entityID),
		zap.String("categoryID", categoryID.String()))

	// If no entityID is provided for entity types that require it, return nil early
	if entityID == nil && entityType != "general" {
		config.Logger.Info("No entityID provided - returning nil (this is normal for new entities)")
		return nil, nil
	}

	var document models.Document

	query := tx.Model(&models.Document{}).
		Where("category_id = ? AND is_current_version = true AND deleted_at IS NULL", categoryID)

	switch entityType {
	case "application":
		query = query.Joins("JOIN application_documents ON documents.id = application_documents.document_id").
			Where("application_documents.application_id = ?", entityID)
	
	case "applicant":
		query = query.Joins("JOIN applicant_documents ON documents.id = applicant_documents.document_id").
			Where("applicant_documents.applicant_id = ?", entityID)
	
	case "stand":
		query = query.Joins("JOIN stand_documents ON documents.id = stand_documents.document_id").
			Where("stand_documents.stand_id = ?", entityID)
	
	case "project":
		query = query.Joins("JOIN project_documents ON documents.id = project_documents.document_id").
			Where("project_documents.project_id = ?", entityID)
	
	case "payment":
		query = query.Joins("JOIN payment_documents ON documents.id = payment_documents.document_id").
			Where("payment_documents.payment_id = ?", entityID)
	
	case "comment":
		query = query.Joins("JOIN comment_documents ON documents.id = comment_documents.document_id").
			Where("comment_documents.comment_id = ?", entityID)
	
	case "email":
		query = query.Joins("JOIN email_documents ON documents.id = email_documents.document_id").
			Where("email_documents.email_log_id = ?", entityID)
	
	case "bank":
		query = query.Joins("JOIN bank_documents ON documents.id = bank_documents.document_id").
			Where("bank_documents.bank_id = ?", entityID)
	
	case "user":
		query = query.Joins("JOIN user_documents ON documents.id = user_documents.document_id").
			Where("user_documents.user_id = ?", entityID)
	
	case "general":
		// For general documents, no additional join needed
		if entityID != nil {
			query = query.Where("id = ?", entityID)
		} else {
			config.Logger.Info("General document with no entityID - returning nil")
			return nil, nil
		}
	
	default:
		config.Logger.Warn("Unknown entity type", zap.String("entityType", entityType))
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	err := query.First(&document).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config.Logger.Info("✅ No existing document found (this is normal for new entities)",
				zap.String("entityType", entityType),
				zap.String("categoryID", categoryID.String()))
			return nil, nil
		}
		config.Logger.Error("❌ Database error in FindExistingDocument", 
			zap.Error(err),
			zap.String("entityType", entityType),
			zap.String("categoryID", categoryID.String()))
		return nil, fmt.Errorf("database error while finding existing document: %w", err)
	}

	config.Logger.Info("📄 Found existing document",
		zap.String("documentID", document.ID.String()),
		zap.Int("version", document.Version),
		zap.String("entityType", entityType))
	
	return &document, nil
}

// UpdateDocument - updates document by document ID
func (r *documentRepository) UpdateDocument(tx *gorm.DB, documentID uuid.UUID, updates map[string]interface{}) ([]models.Document, error) {
	var document models.Document

	// Update the document
	if err := tx.Model(&models.Document{}).Where("id = ?", documentID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update document: %v", err)
	}

	// Reload the document with relationships
	if err := tx.Preload("Category").
		Preload("ApplicantDocuments").
		Preload("ApplicationDocuments").
		Preload("StandDocuments").
		Preload("ProjectDocuments").
		Preload("CommentDocuments").
		Preload("PaymentDocuments").
		Preload("EmailDocuments").
		Preload("BankDocuments").
		Preload("UserDocuments").
		Preload("Previous").
		Preload("Original").
		Preload("Newer").
		Preload("Versions").
		Preload("AuditLogs").
		First(&document, "id = ?", documentID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload document: %v", err)
	}

	return []models.Document{document}, nil
}

// GetDocumentsByPlanID - needs to be updated based on your plan structure
func (r *documentRepository) GetDocumentsByPlanID(planUUID string) ([]models.Document, error) {
	var documents []models.Document

	// This depends on how payment plan documents are stored in your system
	// You might need to create a PaymentPlanDocument join table or use existing relationships
	if err := r.db.Find(&documents).Error; err != nil {
		return nil, err
	}
	return documents, nil
}

func (r *documentRepository) DeleteDocument(id uuid.UUID) error {
	result := r.db.Where("id = ?", id).Delete(&models.Document{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetDocumentsByEntity - get documents for a specific entity using your model structure
func (r *documentRepository) GetDocumentsByEntity(tx *gorm.DB, entityType string, entityID uuid.UUID) ([]models.Document, error) {
	var documents []models.Document

	switch entityType {
	case "applicant":
		err := tx.Joins("JOIN applicant_documents ON documents.id = applicant_documents.document_id").
			Where("applicant_documents.applicant_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	case "application":
		err := tx.Joins("JOIN application_documents ON documents.id = application_documents.document_id").
			Where("application_documents.application_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	case "stand":
		err := tx.Joins("JOIN stand_documents ON documents.id = stand_documents.document_id").
			Where("stand_documents.stand_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	case "project":
		err := tx.Joins("JOIN project_documents ON documents.id = project_documents.document_id").
			Where("project_documents.project_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	case "payment":
		err := tx.Joins("JOIN payment_documents ON documents.id = payment_documents.document_id").
			Where("payment_documents.payment_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	case "comment":
		err := tx.Joins("JOIN comment_documents ON documents.id = comment_documents.document_id").
			Where("comment_documents.comment_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	case "email":
		err := tx.Joins("JOIN email_documents ON documents.id = email_documents.document_id").
			Where("email_documents.email_log_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	case "bank":
		err := tx.Joins("JOIN bank_documents ON documents.id = bank_documents.document_id").
			Where("bank_documents.bank_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	case "user":
		err := tx.Joins("JOIN user_documents ON documents.id = user_documents.document_id").
			Where("user_documents.user_id = ?", entityID).
			Find(&documents).Error
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	return documents, nil
}

// CreateEntityDocumentRelationship - create a join table entry
func (r *documentRepository) CreateEntityDocumentRelationship(tx *gorm.DB, relationship interface{}) error {
	if err := tx.Create(relationship).Error; err != nil {
		return fmt.Errorf("failed to create entity-document relationship: %w", err)
	}
	return nil
}

// DeleteEntityDocumentRelationships - remove all relationships for a document
func (r *documentRepository) DeleteEntityDocumentRelationships(tx *gorm.DB, documentID uuid.UUID) error {
	// Delete from all join tables
	tables := []interface{}{
		&models.ApplicantDocument{},
		&models.ApplicationDocument{},
		&models.StandDocument{},
		&models.ProjectDocument{},
		&models.CommentDocument{},
		&models.PaymentDocument{},
		&models.EmailDocument{},
		&models.BankDocument{},
		&models.UserDocument{},
	}

	for _, table := range tables {
		if err := tx.Where("document_id = ?", documentID).Delete(table).Error; err != nil {
			return fmt.Errorf("failed to delete from %T: %w", table, err)
		}
	}

	return nil
}

// GetDocumentWithRelationships - get document with all its relationships according to your model
func (r *documentRepository) GetDocumentWithRelationships(tx *gorm.DB, documentID uuid.UUID) (*models.Document, error) {
	var document models.Document
	err := tx.Preload("Category").
		Preload("ApplicantDocuments.Applicant").
		Preload("ApplicantDocuments.Application").
		Preload("ApplicationDocuments.Application").
		Preload("StandDocuments.Stand").
		Preload("ProjectDocuments.Project").
		Preload("CommentDocuments.Comment").
		Preload("PaymentDocuments.Payment").
		Preload("EmailDocuments.EmailLog").
		Preload("BankDocuments.Bank").
		Preload("UserDocuments.User").
		Preload("Previous").
		Preload("Original").
		Preload("Newer").
		Preload("Versions").
		Preload("AuditLogs").
		First(&document, "id = ?", documentID).Error

	if err != nil {
		return nil, err
	}
	return &document, nil
}

// repositories/application_repository.go
package repositories

import (
	"fmt"
	"mime/multipart"
	"strings"
	"time"
	"town-planning-backend/applications/requests"
	"town-planning-backend/db/models"
	documents_services "town-planning-backend/documents/services"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ApplicationRepository interface {
	// Development Category methods
	CreateDevelopmentCategory(category *models.DevelopmentCategory) (*models.DevelopmentCategory, error)
	GetDevelopmentCategoryByName(name string) (*models.DevelopmentCategory, error)
	GetFilteredDevelopmentCategories(pageSize, offset int, filters map[string]string) ([]models.DevelopmentCategory, int64, error)
	GetAllDevelopmentCategories(isActive *bool) ([]models.DevelopmentCategory, error)

	// Tariff methods
	CreateTariff(tariff *models.Tariff) (*models.Tariff, error)
	GetActiveTariffForCategory(developmentCategoryID string) (*models.Tariff, error)
	DeactivateTariff(tariffID string, updatedBy string) (*models.Tariff, error)
	GetFilteredDevelopmentTariffs(limit, offset int, filters map[string]string) ([]models.Tariff, int64, error)
	GetTariffByID(tariffID string) (*models.Tariff, error)

	// Application query methods
	GetFilteredApplications(limit, offset int, filters map[string]string) ([]models.Application, int64, error)
	GetApplicationById(applicationID string) (*models.Application, error)
	GetApplicationForUpdate(applicationID string) (*models.Application, error)
	GetApplicationsByStatus(status models.ApplicationStatus, limit, offset int) ([]models.Application, int64, error)

	// Application update methods
	UpdateApplication(tx *gorm.DB, applicationID uuid.UUID, updates map[string]interface{}) (*models.Application, error)
	UpdateApplicationStatus(tx *gorm.DB, applicationID uuid.UUID, status models.ApplicationStatus, updatedBy string) error
	UpdateApplicationArchitect(tx *gorm.DB, applicationID uuid.UUID, architectFullName *string, architectEmail *string, architectPhone *string, updatedBy string) error
	UpdateApplicationDocumentFlags(tx *gorm.DB, applicationID uuid.UUID, documentFlags map[string]bool, updatedBy string) error
	MarkApplicationAsCollected(tx *gorm.DB, applicationID uuid.UUID, collectedBy string, collectionDate *time.Time) error
	ValidateApplicationForUpdate(applicationID uuid.UUID) error

	// Cost calculation methods
	RecalculateApplicationCosts(tx *gorm.DB, applicationID uuid.UUID, tariffID uuid.UUID, vatRateID uuid.UUID, planArea decimal.Decimal) (*CostCalculation, error)

	// Approval group methods
	CreateApprovalGroup(tx *gorm.DB, group *models.ApprovalGroup) (*models.ApprovalGroup, error)
	GetApprovalGroupWithMembers(db *gorm.DB, groupID string) (*models.ApprovalGroup, error)
	GetApprovalGroups(db *gorm.DB) ([]models.ApprovalGroup, error)
	GetApprovalGroupByID(db *gorm.DB, groupID string) (*models.ApprovalGroup, error)
	GetFilteredApprovalGroups(limit, offset int, filters map[string]string) ([]models.ApprovalGroup, int64, error)

	// Approval workflow methods
	GetEnhancedApplicationApprovalData(applicationID string) (*ApplicationApprovalData, error)
	ProcessApplicationApproval(tx *gorm.DB, applicationID string, userID uuid.UUID, comment *string, commentType models.CommentType) (*ApprovalResult, error)
	ProcessApplicationRejection(tx *gorm.DB, applicationID string, userID uuid.UUID, reason string, comment *string, commentType models.CommentType) (*RejectionResult, error)
	RaiseApplicationIssue(tx *gorm.DB, applicationID string, userID uuid.UUID, title string, description string, priority string, category *string, assignmentType models.IssueAssignmentType, assignedToUserID *uuid.UUID, assignedToGroupMemberID *uuid.UUID) (*models.ApplicationIssue, error)
	RaiseApplicationIssueWithChatAndAttachments(tx *gorm.DB, applicationID string, userID uuid.UUID, title string, description string, priority string, category *string, assignmentType models.IssueAssignmentType, assignedToUserID *uuid.UUID, assignedToGroupMemberID *uuid.UUID, attachmentDocumentIDs []uuid.UUID, createdBy string) (*models.ApplicationIssue, *models.ChatThread, error)
	GetChatMessagesWithPreload(threadID string, limit, offset int) ([]FrontendChatMessage, int64, error)
	CreateMessageWithAttachments(tx *gorm.DB, c *fiber.Ctx, threadID string, content string, messageType models.ChatMessageType, senderID uuid.UUID, files []*multipart.FileHeader, applicationID *uuid.UUID, createdBy string) (*EnhancedChatMessage, error)
	AddParticipantToThread(tx *gorm.DB, threadID uuid.UUID, userID uuid.UUID, role models.ParticipantRole, addedBy string) error
	RemoveParticipantFromThread(tx *gorm.DB, threadID uuid.UUID, userID uuid.UUID, removedBy string) error
	CanUserManageParticipants(threadID string, userID uuid.UUID) (bool, error)
	GetThreadParticipants(threadID string) ([]models.ChatParticipant, error)
	RemoveMultipleParticipantsFromThread(tx *gorm.DB, threadID uuid.UUID, userIDs []uuid.UUID, removedBy string) (int, error)
	AddMultipleParticipantsToThread(tx *gorm.DB, threadID uuid.UUID, participants []requests.ParticipantRequest, addedBy string) ([]models.ChatParticipant, error)
	MarkIssueAsResolved(tx *gorm.DB, issueID string, resolvedByUserID uuid.UUID, resolutionComment *string) (*models.ApplicationIssue, error)
	ReopenIssue(tx *gorm.DB, issueID string, reopenedByUserID uuid.UUID) (*models.ApplicationIssue, error)
	GetIssueByID(issueID string) (*models.ApplicationIssue, error)
	DeleteMessage(tx *gorm.DB, messageID uuid.UUID, userID uuid.UUID) error
	StarMessage(tx *gorm.DB, messageID uuid.UUID, userID uuid.UUID) (bool, error)
	CreateReplyMessage(tx *gorm.DB, threadID string, parentMessageID uuid.UUID, content string, messageType models.ChatMessageType, senderID uuid.UUID, files []*multipart.FileHeader, applicationID *uuid.UUID, createdBy string) (*EnhancedChatMessage, error)
	GetMessageStars(messageID uuid.UUID) ([]models.MessageStar, error)
	GetMessageThread(messageID uuid.UUID) ([]*EnhancedChatMessage, error)
	IsMessageStarredByUser(messageID uuid.UUID, userID uuid.UUID) (bool, error)
	GetUnreadMessageCount(threadID string, userID uuid.UUID) (int, error)
	VerifyThreadAccess(tx *gorm.DB, threadID string, userID uuid.UUID) (*models.ChatThread, error)
}

type applicationRepository struct {
	documentSvc *documents_services.DocumentService
	db          *gorm.DB
}

func NewApplicationRepository(db *gorm.DB, documentSvc *documents_services.DocumentService) ApplicationRepository {
	return &applicationRepository{db: db, documentSvc: documentSvc}
}

// verifyThreadAccess verifies the thread exists and user has access
func (ac *applicationRepository) VerifyThreadAccess(tx *gorm.DB, threadID string, userID uuid.UUID) (*models.ChatThread, error) {
	var thread models.ChatThread

	// First, verify thread exists
	if err := tx.Where("id = ? AND is_active = ?", threadID, true).First(&thread).Error; err != nil {
		return nil, fmt.Errorf("thread not found or inactive")
	}

	// Check if user is a participant in this thread
	var participant models.ChatParticipant
	if err := tx.Where("thread_id = ? AND user_id = ? AND is_active = ?", threadID, userID, true).First(&participant).Error; err != nil {
		return nil, fmt.Errorf("user is not a participant in this thread")
	}

	return &thread, nil
}

// CreateApprovalGroup creates a new approval group with its members
func (r *applicationRepository) CreateApprovalGroup(tx *gorm.DB, group *models.ApprovalGroup) (*models.ApprovalGroup, error) {
	// Use transaction to ensure atomicity
	if err := tx.Create(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

// GetApprovalGroupWithMembers fetches an approval group with all its active members and their user details
func (r *applicationRepository) GetApprovalGroupWithMembers(db *gorm.DB, groupID string) (*models.ApprovalGroup, error) {
	var group models.ApprovalGroup

	err := db.
		Preload("Members", "is_active = ?", true).
		Preload("Members.User").
		Preload("Members.User.Department").
		Preload("Members.User.Role").
		Where("id = ?", groupID).
		First(&group).Error

	if err != nil {
		return nil, err
	}

	return &group, nil
}

// GetApprovalGroups fetches all approval groups with basic member info
func (r *applicationRepository) GetApprovalGroups(db *gorm.DB) ([]models.ApprovalGroup, error) {
	var groups []models.ApprovalGroup

	err := db.
		Preload("Members", "is_active = ?", true).
		Preload("Members.User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email")
		}).
		Where("is_active = ?", true).
		Order("created_at DESC").
		Find(&groups).Error

	if err != nil {
		return nil, err
	}

	return groups, nil
}

// GetApprovalGroupByID fetches a single approval group by ID without members
func (r *applicationRepository) GetApprovalGroupByID(db *gorm.DB, groupID string) (*models.ApprovalGroup, error) {
	var group models.ApprovalGroup

	err := db.
		Where("id = ?", groupID).
		First(&group).Error

	if err != nil {
		return nil, err
	}

	return &group, nil
}

func (r *applicationRepository) GetApplicationById(applicationID string) (*models.Application, error) {
	var application models.Application
	if err := r.db.
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Preload("ApplicationDocuments.Document").
		Preload("Payment").
		Preload("ApprovalGroup.Members.User.Department").
		Where("id = ?", applicationID).
		First(&application).Error; err != nil {
		return nil, err
	}
	return &application, nil
}

// GetFilteredApprovalGroups fetches approval groups with filtering and pagination
func (r *applicationRepository) GetFilteredApprovalGroups(limit, offset int, filters map[string]string) ([]models.ApprovalGroup, int64, error) {
	var approvalGroups []models.ApprovalGroup
	var total int64

	// Start building the query with preloads for members and their users
	query := r.db.Model(&models.ApprovalGroup{}).
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("review_order ASC")
		}).
		Preload("Members.User").
		Preload("Assignments", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true)
		})

	// Apply filters
	if name, exists := filters["name"]; exists && name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}

	if groupType, exists := filters["type"]; exists && groupType != "" {
		query = query.Where("type = ?", groupType)
	}

	if isActive, exists := filters["is_active"]; exists && isActive != "" {
		if isActive == "true" {
			query = query.Where("is_active = ?", true)
		} else if isActive == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	if requiresAllApprovals, exists := filters["requires_all_approvals"]; exists && requiresAllApprovals != "" {
		if requiresAllApprovals == "true" {
			query = query.Where("requires_all_approvals = ?", true)
		} else if requiresAllApprovals == "false" {
			query = query.Where("requires_all_approvals = ?", false)
		}
	}

	if autoAssignBackups, exists := filters["auto_assign_backups"]; exists && autoAssignBackups != "" {
		if autoAssignBackups == "true" {
			query = query.Where("auto_assign_backups = ?", true)
		} else if autoAssignBackups == "false" {
			query = query.Where("auto_assign_backups = ?", false)
		}
	}

	if createdBy, exists := filters["created_by"]; exists && createdBy != "" {
		query = query.Where("created_by ILIKE ?", "%"+createdBy+"%")
	}

	// Filter for groups with active members
	if hasActiveMembers, exists := filters["has_active_members"]; exists && hasActiveMembers != "" {
		if hasActiveMembers == "true" {
			query = query.Joins("JOIN approval_group_members ON approval_group_members.approval_group_id = approval_groups.id").
				Where("approval_group_members.is_active = ?", true)
		} else if hasActiveMembers == "false" {
			query = query.Where("NOT EXISTS (?)",
				r.db.Model(&models.ApprovalGroupMember{}).
					Where("approval_group_members.approval_group_id = approval_groups.id").
					Where("approval_group_members.is_active = ?", true),
			)
		}
	}

	// NEW: Filter for groups that have a final approver
	if hasFinalApprover, exists := filters["has_final_approver"]; exists && hasFinalApprover != "" {
		if hasFinalApprover == "true" {
			query = query.Joins("JOIN approval_group_members ON approval_group_members.approval_group_id = approval_groups.id").
				Where("approval_group_members.is_final_approver = ?", true).
				Where("approval_group_members.is_active = ?", true)
		} else if hasFinalApprover == "false" {
			query = query.Where("NOT EXISTS (?)",
				r.db.Model(&models.ApprovalGroupMember{}).
					Where("approval_group_members.approval_group_id = approval_groups.id").
					Where("approval_group_members.is_final_approver = ?", true).
					Where("approval_group_members.is_active = ?", true),
			)
		}
	}

	// Count total number of records matching the filters
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated approval groups, ordered by name
	if err := query.
		Order("name ASC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&approvalGroups).Error; err != nil {
		return nil, 0, err
	}

	return approvalGroups, total, nil
}

// GetFilteredApplications fetches applications with filtering and pagination
func (r *applicationRepository) GetFilteredApplications(limit, offset int, filters map[string]string) ([]models.Application, int64, error) {
	var applications []models.Application
	var total int64

	// Start building the query with preloads
	query := r.db.Model(&models.Application{}).
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate")

	// Apply filters
	if applicantID, exists := filters["applicant_id"]; exists && applicantID != "" {
		query = query.Where("applicant_id = ?", applicantID)
	}

	if planNumber, exists := filters["plan_number"]; exists && planNumber != "" {
		query = query.Where("plan_number ILIKE ?", "%"+planNumber+"%")
	}

	if permitNumber, exists := filters["permit_number"]; exists && permitNumber != "" {
		query = query.Where("permit_number ILIKE ?", "%"+permitNumber+"%")
	}

	if status, exists := filters["status"]; exists && status != "" {
		query = query.Where("status = ?", status)
	}

	if paymentStatus, exists := filters["payment_status"]; exists && paymentStatus != "" {
		query = query.Where("payment_status = ?", paymentStatus)
	}

	if standID, exists := filters["stand_id"]; exists && standID != "" {
		query = query.Where("stand_id = ?", standID)
	}

	if architectName, exists := filters["architect_name"]; exists && architectName != "" {
		query = query.Where("architect_full_name ILIKE ?", "%"+architectName+"%")
	}

	if dateFrom, exists := filters["date_from"]; exists && dateFrom != "" {
		parsedDate, err := time.Parse("2006-01-02", dateFrom)
		if err == nil {
			query = query.Where("submission_date >= ?", parsedDate)
		}
	}

	if dateTo, exists := filters["date_to"]; exists && dateTo != "" {
		parsedDate, err := time.Parse("2006-01-02", dateTo)
		if err == nil {
			// Add one day to include the entire end date
			parsedDate = parsedDate.Add(24 * time.Hour)
			query = query.Where("submission_date < ?", parsedDate)
		}
	}

	if isCollected, exists := filters["is_collected"]; exists && isCollected != "" {
		if isCollected == "true" {
			query = query.Where("is_collected = ?", true)
		} else if isCollected == "false" {
			query = query.Where("is_collected = ?", false)
		}
	}

	// Count total number of records matching the filters
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated applications, ordered by submission date (descending) to show latest first
	if err := query.
		Order("submission_date DESC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&applications).Error; err != nil {
		return nil, 0, err
	}

	return applications, total, nil
}

// GetTariffByID fetches a tariff by ID
func (r *applicationRepository) GetTariffByID(tariffID string) (*models.Tariff, error) {
	var tariff models.Tariff
	if err := r.db.Preload("DevelopmentCategory").First(&tariff, tariffID).Error; err != nil {
		return nil, err
	}
	return &tariff, nil
}

// GetFilteredDevelopmentTariffs fetches tariffs with filtering and pagination
func (r *applicationRepository) GetFilteredDevelopmentTariffs(limit, offset int, filters map[string]string) ([]models.Tariff, int64, error) {
	var tariffs []models.Tariff
	var total int64

	// Start building the query
	query := r.db.Model(&models.Tariff{}).Preload("DevelopmentCategory")

	// Apply filters
	if developmentCategoryID, exists := filters["development_category_id"]; exists && developmentCategoryID != "" {
		query = query.Where("development_category_id = ?", developmentCategoryID)
	}

	if isActive, exists := filters["is_active"]; exists && isActive != "" {
		if isActive == "true" {
			query = query.Where("is_active = ?", true)
		} else if isActive == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	// Count total number of records matching the filters
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated tariffs, ordered by ValidFrom (descending) to show latest first
	if err := query.
		Order("valid_from DESC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&tariffs).Error; err != nil {
		return nil, 0, err
	}

	return tariffs, total, nil
}

// CreateDevelopmentCategory creates a new development category
func (r *applicationRepository) CreateDevelopmentCategory(category *models.DevelopmentCategory) (*models.DevelopmentCategory, error) {
	if err := r.db.Create(category).Error; err != nil {
		return nil, err
	}
	return category, nil
}

// GetDevelopmentCategoryByName finds a development category by name
func (r *applicationRepository) GetDevelopmentCategoryByName(name string) (*models.DevelopmentCategory, error) {
	var category models.DevelopmentCategory

	cleanName := strings.ToUpper(strings.TrimSpace(name))

	if err := r.db.Where("UPPER(TRIM(name)) = ? AND is_active = ?", cleanName, true).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

// CreateTariff creates a new tariff
func (r *applicationRepository) CreateTariff(tariff *models.Tariff) (*models.Tariff, error) {
	if err := r.db.Create(tariff).Error; err != nil {
		return nil, err
	}

	// Preload the development category relationship
	if err := r.db.Preload("DevelopmentCategory").First(tariff, tariff.ID).Error; err != nil {
		return nil, err
	}

	return tariff, nil
}

// GetActiveTariffForCategory gets the currently active tariff for a category
func (r *applicationRepository) GetActiveTariffForCategory(developmentCategoryID string) (*models.Tariff, error) {
	var tariff models.Tariff

	now := time.Now()
	err := r.db.Where("development_category_id = ? AND is_active = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)",
		developmentCategoryID, true, now, now).
		Order("valid_from DESC").
		First(&tariff).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &tariff, nil
}

// DeactivateTariff deactivates a tariff
func (r *applicationRepository) DeactivateTariff(tariffID string, updatedBy string) (*models.Tariff, error) {
	var tariff models.Tariff

	if err := r.db.Where("id = ?", tariffID).First(&tariff).Error; err != nil {
		return nil, err
	}

	tariff.IsActive = false
	tariff.UpdatedAt = time.Now()

	if err := r.db.Save(&tariff).Error; err != nil {
		return nil, err
	}

	// Preload the development category for the response
	if err := r.db.Preload("DevelopmentCategory").First(&tariff, tariff.ID).Error; err != nil {
		return nil, err
	}

	return &tariff, nil
}

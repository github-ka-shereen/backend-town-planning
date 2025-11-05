package repositories

import (
	"errors"
	"fmt"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ApplicantRepository interface {
	CreateApplicant(tx *gorm.DB, applicant *models.Applicant) (*models.Applicant, error)
	GetAllApplicants() ([]models.Applicant, error)
	GetFilteredApplicants(limit, offset int) ([]models.Applicant, int64, error)
	GetActiveVATRate(tx *gorm.DB) (*models.VATRate, error)
	DeactivateVATRate(tx *gorm.DB, vatRateID uuid.UUID, createdBy string) (*models.VATRate, error)
	CreateVATRate(tx *gorm.DB, vatRate *models.VATRate) (*models.VATRate, error)
	GetFilteredVatRates(limit, offset int, filters map[string]string) ([]models.VATRate, int64, error)
	AssignApplicationToGroup(tx *gorm.DB, applicationID string, groupID uuid.UUID, assignedBy string, reassignReason *string, userUUID uuid.UUID) (*models.ApplicationGroupAssignment, error)
}

type applicantRepository struct {
	DB *gorm.DB
}

// NewApplicantRepository initializes a new applicant repository
func NewApplicantRepository(db *gorm.DB) ApplicantRepository {
	return &applicantRepository{DB: db}
}

// AssignApplicationToGroup assigns or reassigns an application to an approval group for review
func (r *applicantRepository) AssignApplicationToGroup(tx *gorm.DB, applicationID string, groupID uuid.UUID, assignedBy string, reassignReason *string, userUUID uuid.UUID) (*models.ApplicationGroupAssignment, error) {
	config.Logger.Info("🔍 AssignApplicationToGroup starting", 
		zap.String("applicationID", applicationID), 
		zap.String("groupID", groupID.String()),
		zap.String("assignedBy", assignedBy),
		zap.String("userUUID", userUUID.String()))

	// Fetch the application and group to validate
	var application models.Application
	var group models.ApprovalGroup

	config.Logger.Info("🔍 Looking up application", zap.String("applicationID", applicationID))
	if err := tx.Where("id = ?", applicationID).First(&application).Error; err != nil {
		config.Logger.Error("❌ Application not found", zap.String("applicationID", applicationID), zap.Error(err))
		return nil, fmt.Errorf("application not found: %w", err)
	}
	config.Logger.Info("✅ Application found", 
		zap.String("applicationID", application.ID.String()),
		zap.String("applicationStatus", string(application.Status)))

	config.Logger.Info("🔍 Looking up approval group", zap.String("groupID", groupID.String()))
	if err := tx.Where("id = ?", groupID).First(&group).Error; err != nil {
		config.Logger.Error("❌ Approval group not found", zap.String("groupID", groupID.String()), zap.Error(err))
		return nil, fmt.Errorf("approval group not found: %w", err)
	}
	config.Logger.Info("✅ Approval group found", 
		zap.String("groupID", group.ID.String()),
		zap.String("groupName", group.Name))

	// Check for existing active assignment
	var existingAssignment models.ApplicationGroupAssignment

	config.Logger.Info("🔍 Checking for existing active assignments", zap.String("applicationID", applicationID))
	if err := tx.Where("application_id = ? AND is_active = ?", applicationID, true).First(&existingAssignment).Error; err == nil {
		config.Logger.Info("🔄 Found existing active assignment, processing reassignment",
			zap.String("existingAssignmentID", existingAssignment.ID.String()),
			zap.String("existingGroupID", existingAssignment.ApprovalGroupID.String()))

		completedAt := time.Now()
		// Deactivate the existing assignment
		existingAssignment.IsActive = false
		existingAssignment.CompletedAt = &completedAt
		
		config.Logger.Info("🔍 Deactivating existing assignment")
		if err := tx.Save(&existingAssignment).Error; err != nil {
			config.Logger.Error("❌ Failed to deactivate existing assignment", 
				zap.String("assignmentID", existingAssignment.ID.String()),
				zap.Error(err))
			return nil, fmt.Errorf("failed to deactivate existing assignment: %w", err)
		}
		config.Logger.Info("✅ Existing assignment deactivated")

		// Create a comment about the reassignment
		comment := models.Comment{
			ID:            uuid.New(),
			ApplicationID: application.ID,
			CommentType:   models.CommentTypeGeneral,
			Content: fmt.Sprintf("Application reassigned from group '%s' to '%s'. Reason: %s",
				existingAssignment.Group.Name, group.Name,
				reassignReasonOrDefault(reassignReason)),
			UserID:    userUUID,
			CreatedBy: assignedBy,
		}
		
		config.Logger.Info("🔍 Creating reassignment comment")
		if err := tx.Create(&comment).Error; err != nil {
			// Log but don't fail the operation
			config.Logger.Warn("⚠️ Failed to create reassignment comment", 
				zap.String("commentID", comment.ID.String()),
				zap.Error(err))
		} else {
			config.Logger.Info("✅ Reassignment comment created")
		}
	} else {
		config.Logger.Info("✅ No existing active assignment found (this is normal for new applications)")
	}

	// Count active group members for the new group
	var memberCount int64
	config.Logger.Info("🔍 Counting active group members", zap.String("groupID", groupID.String()))
	if err := tx.Model(&models.ApprovalGroupMember{}).
		Where("approval_group_id = ? AND is_active = ?", groupID, true).
		Count(&memberCount).Error; err != nil {
		config.Logger.Error("❌ Failed to count group members", 
			zap.String("groupID", groupID.String()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to count group members: %w", err)
	}
	config.Logger.Info("✅ Group member count", 
		zap.String("groupID", groupID.String()),
		zap.Int64("memberCount", memberCount))

	// Create the new assignment
	assignment := models.ApplicationGroupAssignment{
		ID:                    uuid.New(),
		ApplicationID:         application.ID,
		ApprovalGroupID:       groupID,
		IsActive:              true,
		AssignedAt:            time.Now(),
		AssignedBy:            assignedBy,
		TotalMembers:          int(memberCount),
		AvailableMembers:      int(memberCount),
		PendingCount:          int(memberCount),
		ApprovedCount:         0, // Reset counters for new assignment
		RejectedCount:         0,
		IssuesRaised:          0,
		IssuesResolved:        0,
		ReadyForFinalApproval: false,
		UsedBackupMembers:     false,
	}

	config.Logger.Info("🔍 Creating new group assignment", 
		zap.String("assignmentID", assignment.ID.String()),
		zap.String("applicationID", assignment.ApplicationID.String()),
		zap.String("groupID", assignment.ApprovalGroupID.String()))
	
	if err := tx.Create(&assignment).Error; err != nil {
		config.Logger.Error("❌ Failed to create group assignment", 
			zap.String("assignmentID", assignment.ID.String()),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create group assignment: %w", err)
	}
	config.Logger.Info("✅ New group assignment created")

	// Update application's assigned group and status
	updates := map[string]interface{}{
		"assigned_group_id": groupID,
		"status":            models.UnderReviewApplication,
	}

	config.Logger.Info("🔍 Updating application status and assigned group",
		zap.String("applicationID", application.ID.String()),
		zap.Any("updates", updates))
	
	if err := tx.Model(&application).Updates(updates).Error; err != nil {
		config.Logger.Error("❌ Failed to update application", 
			zap.String("applicationID", application.ID.String()),
			zap.Any("updates", updates),
			zap.Error(err))
		return nil, fmt.Errorf("failed to update application: %w", err)
	}
	config.Logger.Info("✅ Application updated successfully")

	config.Logger.Info("🎉 AssignApplicationToGroup completed successfully",
		zap.String("assignmentID", assignment.ID.String()),
		zap.String("applicationID", application.ID.String()),
		zap.String("groupID", groupID.String()))
	
	return &assignment, nil
}

// // AssignApplicationToGroup assigns or reassigns an application to an approval group for review
// func (r *applicantRepository) AssignApplicationToGroup(tx *gorm.DB, applicationID string, groupID uuid.UUID, assignedBy string, reassignReason *string, userUUID uuid.UUID) (*models.ApplicationGroupAssignment, error) {
// 	// Fetch the application and group to validate
// 	var application models.Application
// 	var group models.ApprovalGroup

// 	if err := tx.Where("id = ?", applicationID).First(&application).Error; err != nil {
// 		return nil, fmt.Errorf("application not found: %w", err)
// 	}

// 	if err := tx.Where("id = ?", groupID).First(&group).Error; err != nil {
// 		return nil, fmt.Errorf("approval group not found: %w", err)
// 	}

// 	// Check for existing active assignment
// 	var existingAssignment models.ApplicationGroupAssignment

// 	if err := tx.Where("application_id = ? AND is_active = ?", applicationID, true).First(&existingAssignment).Error; err == nil {

// 		completedAt := time.Now()
// 		// Deactivate the existing assignment
// 		existingAssignment.IsActive = false
// 		existingAssignment.CompletedAt = &completedAt
// 		if err := tx.Save(&existingAssignment).Error; err != nil {
// 			return nil, fmt.Errorf("failed to deactivate existing assignment: %w", err)
// 		}

// 		// Create a comment about the reassignment
// 		comment := models.Comment{
// 			ID:            uuid.New(),
// 			ApplicationID: application.ID,
// 			CommentType:   models.CommentTypeGeneral,
// 			Content: fmt.Sprintf("Application reassigned from group '%s' to '%s'. Reason: %s",
// 				existingAssignment.Group.Name, group.Name,
// 				reassignReasonOrDefault(reassignReason)),
// 			UserID:    userUUID,
// 			CreatedBy: assignedBy,
// 		}
// 		if err := tx.Create(&comment).Error; err != nil {
// 			// Log but don't fail the operation
// 			config.Logger.Warn("Failed to create reassignment comment", zap.Error(err))
// 		}
// 	}

// 	// Count active group members for the new group
// 	var memberCount int64
// 	if err := tx.Model(&models.ApprovalGroupMember{}).
// 		Where("approval_group_id = ? AND is_active = ?", groupID, true).
// 		Count(&memberCount).Error; err != nil {
// 		return nil, fmt.Errorf("failed to count group members: %w", err)
// 	}

// 	// Create the new assignment
// 	assignment := models.ApplicationGroupAssignment{
// 		ID:                    uuid.New(),
// 		ApplicationID:         application.ID,
// 		ApprovalGroupID:       groupID,
// 		IsActive:              true,
// 		AssignedAt:            time.Now(),
// 		AssignedBy:            assignedBy,
// 		TotalMembers:          int(memberCount),
// 		AvailableMembers:      int(memberCount),
// 		PendingCount:          int(memberCount),
// 		ApprovedCount:         0, // Reset counters for new assignment
// 		RejectedCount:         0,
// 		IssuesRaised:          0,
// 		IssuesResolved:        0,
// 		ReadyForFinalApproval: false,
// 		UsedBackupMembers:     false,
// 	}

// 	if err := tx.Create(&assignment).Error; err != nil {
// 		return nil, fmt.Errorf("failed to create group assignment: %w", err)
// 	}

// 	// Update application's assigned group and status
// 	updates := map[string]interface{}{
// 		"assigned_group_id": groupID,
// 		"status":            models.UnderReviewApplication,
// 	}

// 	if err := tx.Model(&application).Updates(updates).Error; err != nil {
// 		return nil, fmt.Errorf("failed to update application: %w", err)
// 	}

// 	return &assignment, nil
// }

func reassignReasonOrDefault(reason *string) string {
	if reason != nil && *reason != "" {
		return *reason
	}
	return "No reason provided"
}

func (ar *applicantRepository) GetAllApplicants() ([]models.Applicant, error) {
	var applicants []models.Applicant
	if err := ar.DB.Find(&applicants).Error; err != nil {
		config.Logger.Error("Failed to get all applicants", zap.Error(err))
		return nil, fmt.Errorf("failed to get all applicants: %w", err)
	}
	return applicants, nil
}

func (ar *applicantRepository) GetFilteredApplicants(limit, offset int) ([]models.Applicant, int64, error) {
	var applicants []models.Applicant
	var total int64

	// Count total number of applicants
	if err := ar.DB.Model(&models.Applicant{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated applicants, ordered by UpdatedAt and CreatedAt (descending)
	if err := ar.DB.Order("updated_at DESC, created_at DESC").Limit(limit).Offset(offset).Find(&applicants).Error; err != nil {
		return nil, 0, err
	}

	return applicants, total, nil
}

func (ar *applicantRepository) CreateApplicant(tx *gorm.DB, applicant *models.Applicant) (*models.Applicant, error) {
	// Set full name based on applicant type
	switch applicant.ApplicantType {
	case models.IndividualApplicant:
		applicant.FullName = applicant.GetFullName()
		config.Logger.Info("Set full name for individual applicant",
			zap.String("fullName", applicant.FullName))

	case models.OrganisationApplicant:
		if applicant.OrganisationName == nil {
			return nil, errors.New("organisation name is required for organisation applicants")
		}
		applicant.FullName = *applicant.OrganisationName
		config.Logger.Info("Set full name for organisation applicant",
			zap.String("organisationName", *applicant.OrganisationName))
	}

	// Set default status if not provided
	if applicant.Status == "" {
		applicant.Status = models.ProspectiveApplicant
		config.Logger.Info("Applicant status set to default",
			zap.String("status", string(applicant.Status)))
	}

	// Create the applicant with associations
	if err := tx.Create(applicant).Error; err != nil {
		config.Logger.Error("Failed to create applicant",
			zap.Error(err),
			zap.Any("applicantData", applicant))
		return nil, fmt.Errorf("failed to create applicant: %w", err)
	}

	// If you need to load the relationships after creation:
	if err := tx.Preload("OrganisationRepresentatives").
		Preload("AdditionalPhoneNumbers").
		First(applicant, applicant.ID).Error; err != nil {
		config.Logger.Error("Failed to load applicant relationships",
			zap.Error(err),
			zap.String("applicantID", applicant.ID.String()))
		return nil, fmt.Errorf("failed to load applicant relationships: %w", err)
	}

	config.Logger.Info("Created applicant successfully",
		zap.String("applicantID", applicant.ID.String()),
		zap.Int("representatives", len(applicant.OrganisationRepresentatives)),
		zap.Int("phoneNumbers", len(applicant.AdditionalPhoneNumbers)))

	return applicant, nil
}

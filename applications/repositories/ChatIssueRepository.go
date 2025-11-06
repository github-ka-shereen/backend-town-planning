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

// RaiseApplicationIssueWithChatAndAttachments raises an issue with chat thread and optional pre-processed attachments
func (r *applicationRepository) RaiseApplicationIssueWithChatAndAttachments(
	tx *gorm.DB,
	applicationID string,
	userID uuid.UUID,
	title string,
	description string,
	priority string,
	category *string,
	assignmentType models.IssueAssignmentType,
	assignedToUserID *uuid.UUID,
	assignedToGroupMemberID *uuid.UUID,
	attachmentDocumentIDs []uuid.UUID,
	createdBy string,
) (*models.ApplicationIssue, *models.ChatThread, error) {
	// Fetch application with group assignment and members
	var application models.Application
	err := tx.
		Preload("ApprovalGroup.Members", "is_active = ?", true).
		Preload("GroupAssignments", "is_active = ?", true).
		Where("id = ?", applicationID).
		First(&application).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("application not found")
		}
		return nil, nil, fmt.Errorf("failed to fetch application: %w", err)
	}

	// Validate we have an approval group
	if application.ApprovalGroup == nil {
		return nil, nil, errors.New("application has no approval group")
	}

	// Check if user is an active member of the approval group
	var groupMember models.ApprovalGroupMember
	err = tx.
		Preload("User").
		Where("approval_group_id = ? AND user_id = ? AND is_active = ?",
			application.ApprovalGroup.ID, userID, true).
		First(&groupMember).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("user not authorized to raise issues for this application")
		}
		return nil, nil, fmt.Errorf("failed to fetch group member: %w", err)
	}

	// Check if user can raise issues
	if !groupMember.CanRaiseIssues {
		return nil, nil, errors.New("user does not have permission to raise issues")
	}

	// Check if there's an active group assignment
	if len(application.GroupAssignments) == 0 {
		return nil, nil, errors.New("no active group assignment found for this application")
	}

	assignment := application.GroupAssignments[0]

	// Validate assignment based on assignment type
	tempIssue := models.ApplicationIssue{
		AssignmentType:          assignmentType,
		AssignedToUserID:        assignedToUserID,
		AssignedToGroupMemberID: assignedToGroupMemberID,
	}

	if err := tempIssue.ValidateAssignment(); err != nil {
		return nil, nil, fmt.Errorf("invalid assignment: %w", err)
	}

	// Additional validation specific to the context
	switch assignmentType {
	case models.IssueAssignment_GROUP_MEMBER:
		// Verify the assigned member belongs to the same group and is active
		var assignedMember models.ApprovalGroupMember
		if err := tx.
			Where("id = ? AND approval_group_id = ? AND is_active = ?",
				assignedToGroupMemberID, application.ApprovalGroup.ID, true).
			First(&assignedMember).Error; err != nil {
			return nil, nil, errors.New("invalid group member assignment - member not found or inactive")
		}
		if !assignedMember.CanApprove && !assignedMember.CanReject {
			return nil, nil, errors.New("assigned group member does not have resolution permissions")
		}

	case models.IssueAssignment_SPECIFIC_USER:
		// Verify user exists and is active
		var assignedUser models.User
		if err := tx.Where("id = ? AND is_active = ?", assignedToUserID, true).First(&assignedUser).Error; err != nil {
			return nil, nil, errors.New("invalid user assignment - user not found or inactive")
		}
	}

	// ========================================
	// CREATE DECISION RECORD FIRST
	// ========================================
	decisionID := uuid.New()
	decision := models.MemberApprovalDecision{
		ID:                      decisionID,
		AssignmentID:            assignment.ID,
		MemberID:                groupMember.ID,
		UserID:                  userID,
		Status:                  models.DecisionPending,
		AssignedAs:              groupMember.Role,
		IsFinalApproverDecision: groupMember.IsFinalApprover,
		WasAvailable:            groupMember.AvailabilityStatus == models.AvailabilityAvailable,
	}

	if err := tx.Create(&decision).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create decision record: %w", err)
	}

	// ========================================
	// CREATE THE ISSUE FIRST (WITHOUT CHAT THREAD REFERENCE)
	// ========================================
	issue := models.ApplicationIssue{
		ID:                      uuid.New(),
		ApplicationID:           application.ID,
		AssignmentID:            assignment.ID,
		RaisedByDecisionID:      decisionID,
		RaisedByUserID:          userID,
		AssignmentType:          assignmentType,
		AssignedToUserID:        assignedToUserID,
		AssignedToGroupMemberID: assignedToGroupMemberID,
		ChatThreadID:            nil, // Will be set after chat thread creation
		Title:                   title,
		Description:             description,
		Priority:                priority,
		Category:                category,
		IsResolved:              false,
	}

	if err := tx.Create(&issue).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create issue: %w", err)
	}

	// ========================================
	// CREATE CHAT THREAD WITH THE VALID ISSUE ID
	// ========================================
	chatThread, err := r.createChatThreadForIssue(
		tx,
		&application,
		&assignment,
		&groupMember,
		&issue, // Pass the created issue
		title,
		description,
		assignmentType,
		assignedToUserID,
		assignedToGroupMemberID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create chat thread: %w", err)
	}

	// ========================================
	// UPDATE ISSUE WITH CHAT THREAD ID
	// ========================================
	issue.ChatThreadID = &chatThread.ID
	if err := tx.Save(&issue).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to update issue with chat thread ID: %w", err)
	}

	// ========================================
	// CREATE INITIAL CHAT MESSAGE WITH OPTIONAL ATTACHMENTS
	// ========================================
	_, err = r.createInitialChatMessageWithAttachments(
		tx,
		chatThread,
		&groupMember,
		description,
		attachmentDocumentIDs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create initial chat message with attachments: %w", err)
	}

	// ========================================
	// UPDATE ASSIGNMENT COUNTS
	// ========================================
	assignment.IssuesRaised++
	if err := tx.Save(&assignment).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to update assignment issue count: %w", err)
	}

	// Update final approval status if needed
	if assignment.ReadyForFinalApproval && !assignment.IsReadyForFinalApproval() {
		assignment.ReadyForFinalApproval = false
		if err := tx.Save(&assignment).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to update final approval status: %w", err)
		}
	}

	config.Logger.Info("Issue raised successfully with optional attachments",
		zap.String("applicationID", applicationID),
		zap.String("issueID", issue.ID.String()),
		zap.String("chatThreadID", chatThread.ID.String()),
		zap.Int("attachmentCount", len(attachmentDocumentIDs)))

	return &issue, chatThread, nil
}

// createChatThreadForIssue creates a chat thread with appropriate participants
func (r *applicationRepository) createChatThreadForIssue(
	tx *gorm.DB,
	application *models.Application,
	assignment *models.ApplicationGroupAssignment,
	raisedByMember *models.ApprovalGroupMember,
	issue *models.ApplicationIssue, // Pass the created issue
	title string,
	description string,
	assignmentType models.IssueAssignmentType,
	assignedToUserID *uuid.UUID,
	assignedToGroupMemberID *uuid.UUID,
) (*models.ChatThread, error) {

	// Determine thread type and participants based on assignment type
	var threadType models.ChatThreadType
	var participants []models.ChatParticipant

	switch assignmentType {
	case models.IssueAssignment_COLLABORATIVE:
		threadType = models.ChatThreadGroup
		participants = r.getGroupParticipants(tx, application.ApprovalGroup, raisedByMember.UserID)

	case models.IssueAssignment_GROUP_MEMBER:
		threadType = models.ChatThreadMixed
		participants = r.getGroupMemberParticipants(tx, application.ApprovalGroup, raisedByMember.UserID, assignedToGroupMemberID)

	case models.IssueAssignment_SPECIFIC_USER:
		threadType = models.ChatThreadSpecificUser
		participants = r.getSpecificUserParticipants(tx, raisedByMember.UserID, assignedToUserID)
	}

	// Create the chat thread WITH THE VALID ISSUE ID
	chatThread := models.ChatThread{
		ID:              uuid.New(),
		ApplicationID:   application.ID,
		IssueID:         issue.ID, // Use the created issue's ID
		ThreadType:      threadType,
		Title:           title,
		Description:     &description,
		CreatedByUserID: raisedByMember.UserID,
		IsActive:        true,
		IsResolved:      false,
	}

	if err := tx.Create(&chatThread).Error; err != nil {
		return nil, fmt.Errorf("failed to create chat thread: %w", err)
	}

	// Update participants with the actual thread ID
	for i := range participants {
		participants[i].ThreadID = chatThread.ID
	}

	// Create participants
	for _, participant := range participants {
		if err := tx.Create(&participant).Error; err != nil {
			config.Logger.Warn("Failed to create chat participant, continuing",
				zap.Error(err),
				zap.String("userID", participant.UserID.String()))
			// Continue with other participants
		}
	}

	config.Logger.Info("Chat thread created successfully",
		zap.String("chatThreadID", chatThread.ID.String()),
		zap.String("threadType", string(threadType)),
		zap.Int("participantCount", len(participants)))

	return &chatThread, nil
}

// createInitialChatMessageWithAttachments creates the initial chat message with optional file attachments
func (r *applicationRepository) createInitialChatMessageWithAttachments(
	tx *gorm.DB,
	chatThread *models.ChatThread,
	raisedByMember *models.ApprovalGroupMember,
	description string,
	attachmentDocumentIDs []uuid.UUID,
) (*models.ChatMessage, error) {

	// Create the initial chat message
	initialMessage := models.ChatMessage{
		ID:          uuid.New(),
		ThreadID:    chatThread.ID,
		SenderID:    raisedByMember.UserID,
		Content:     fmt.Sprintf("Issue created: %s", description),
		MessageType: models.MessageTypeSystem,
		Status:      models.MessageStatusSent,
	}

	if err := tx.Create(&initialMessage).Error; err != nil {
		return nil, fmt.Errorf("failed to create initial chat message: %w", err)
	}

	// Process file attachments if any are provided
	if len(attachmentDocumentIDs) > 0 {
		if err := r.linkChatMessageAttachments(tx, &initialMessage, attachmentDocumentIDs); err != nil {
			config.Logger.Warn("Failed to link some attachments, continuing with issue creation",
				zap.Error(err),
				zap.String("messageID", initialMessage.ID.String()),
				zap.Int("totalAttachments", len(attachmentDocumentIDs)))
			// Don't fail the entire operation if attachment linking fails
		}
	} else {
		config.Logger.Info("No attachments provided for initial chat message",
			zap.String("messageID", initialMessage.ID.String()))
	}

	return &initialMessage, nil
}

// linkChatMessageAttachments links existing documents to a chat message
func (r *applicationRepository) linkChatMessageAttachments(
	tx *gorm.DB,
	chatMessage *models.ChatMessage,
	documentIDs []uuid.UUID,
) error {

	successCount := 0
	for _, documentID := range documentIDs {
		// Create chat attachment relationship
		chatAttachment := models.ChatAttachment{
			ID:         uuid.New(),
			MessageID:  chatMessage.ID,
			DocumentID: documentID,
		}

		if err := tx.Create(&chatAttachment).Error; err != nil {
			config.Logger.Error("Failed to create chat attachment relationship",
				zap.Error(err),
				zap.String("documentID", documentID.String()),
				zap.String("messageID", chatMessage.ID.String()))
			continue
		}

		successCount++
		config.Logger.Debug("Chat attachment linked successfully",
			zap.String("documentID", documentID.String()),
			zap.String("messageID", chatMessage.ID.String()))
	}

	config.Logger.Info("Chat attachments linking completed",
		zap.Int("successful", successCount),
		zap.Int("failed", len(documentIDs)-successCount),
		zap.String("messageID", chatMessage.ID.String()))

	return nil
}

// Helper methods for participant management (unchanged)
func (r *applicationRepository) getGroupParticipants(tx *gorm.DB, group *models.ApprovalGroup, raisedByUserID uuid.UUID) []models.ChatParticipant {
	var participants []models.ChatParticipant

	for _, member := range group.Members {
		if member.IsActive {
			role := models.ParticipantRoleMember
			if member.UserID == raisedByUserID {
				role = models.ParticipantRoleOwner
			}

			participants = append(participants, models.ChatParticipant{
				ID:        uuid.New(),
				ThreadID:  uuid.Nil, // Will be set after thread creation
				UserID:    member.UserID,
				Role:      role,
				IsActive:  true,
				CanInvite: role == models.ParticipantRoleOwner || role == models.ParticipantRoleAdmin,
				AddedBy:   "system",
				AddedAt:   time.Now(),
			})
		}
	}

	config.Logger.Debug("Group participants determined",
		zap.Int("totalParticipants", len(participants)),
		zap.String("raisedByUserID", raisedByUserID.String()))

	return participants
}

func (r *applicationRepository) getGroupMemberParticipants(tx *gorm.DB, group *models.ApprovalGroup, raisedByUserID uuid.UUID, assignedToMemberID *uuid.UUID) []models.ChatParticipant {
	var participants []models.ChatParticipant

	// Add the issue raiser
	participants = append(participants, models.ChatParticipant{
		ID:        uuid.New(),
		ThreadID:  uuid.Nil,
		UserID:    raisedByUserID,
		Role:      models.ParticipantRoleOwner,
		IsActive:  true,
		CanInvite: true,
		AddedBy:   "system",
		AddedAt:   time.Now(),
	})

	// Add the assigned group member
	if assignedToMemberID != nil {
		var assignedMember models.ApprovalGroupMember
		if err := tx.Where("id = ?", assignedToMemberID).First(&assignedMember).Error; err == nil {
			participants = append(participants, models.ChatParticipant{
				ID:        uuid.New(),
				ThreadID:  uuid.Nil,
				UserID:    assignedMember.UserID,
				Role:      models.ParticipantRoleAdmin,
				IsActive:  true,
				CanInvite: true,
				AddedBy:   "system",
				AddedAt:   time.Now(),
			})
			config.Logger.Debug("Added assigned group member to participants",
				zap.String("assignedMemberID", assignedMember.ID.String()),
				zap.String("userID", assignedMember.UserID.String()))
		} else {
			config.Logger.Warn("Assigned group member not found, proceeding without them",
				zap.String("assignedMemberID", assignedToMemberID.String()),
				zap.Error(err))
		}
	}

	return participants
}

func (r *applicationRepository) getSpecificUserParticipants(tx *gorm.DB, raisedByUserID uuid.UUID, assignedToUserID *uuid.UUID) []models.ChatParticipant {
	var participants []models.ChatParticipant

	// Add the issue raiser
	participants = append(participants, models.ChatParticipant{
		ID:        uuid.New(),
		ThreadID:  uuid.Nil,
		UserID:    raisedByUserID,
		Role:      models.ParticipantRoleOwner,
		IsActive:  true,
		CanInvite: true,
		AddedBy:   "system",
		AddedAt:   time.Now(),
	})

	// Add the assigned user
	if assignedToUserID != nil {
		participants = append(participants, models.ChatParticipant{
			ID:        uuid.New(),
			ThreadID:  uuid.Nil,
			UserID:    *assignedToUserID,
			Role:      models.ParticipantRoleAdmin,
			IsActive:  true,
			CanInvite: true,
			AddedBy:   "system",
			AddedAt:   time.Now(),
		})
		config.Logger.Debug("Added assigned user to participants",
			zap.String("assignedUserID", assignedToUserID.String()))
	}

	return participants
}

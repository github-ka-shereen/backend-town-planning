// repositories/application_repository.go

package repositories

import (
	"time"
	"town-planning-backend/db/models"
	"town-planning-backend/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkflowStatus struct {
	CurrentStage        string     `json:"current_stage"`
	PreviousStages      []string   `json:"previous_stages"`
	NextStages          []string   `json:"next_stages"`
	EstimatedCompletion *time.Time `json:"estimated_completion"`
	TotalDepartments    int        `json:"total_departments"`
	ApprovedDepartments int        `json:"approved_departments"`
	ProgressPercentage  int        `json:"progress_percentage"`
}

type ChatParticipantSummary struct {
	ID        uuid.UUID `json:"id"`
	FullName  string    `json:"full_name"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	AvatarURL string    `json:"avatar_url"`
}

// Enhanced ApplicationApprovalData with all required fields
type ApplicationApprovalData struct {
	Application      *EnhancedApplicationView `json:"application"`
	ApprovalProgress int                      `json:"approval_progress"`
	CanTakeAction    bool                     `json:"can_take_action"`
	UnresolvedIssues int                      `json:"unresolved_issues"`
	Workflow         *WorkflowStatus          `json:"workflow"`
	ChatThreads      []*EnhancedChatThread    `json:"chat_threads,omitempty"`
}

// EnhancedApplicationView includes all fields needed by frontend
type EnhancedApplicationView struct {
	// Basic application info
	ID                   uuid.UUID                `json:"id"`
	PlanNumber           string                   `json:"plan_number"`
	PermitNumber         string                   `json:"permit_number"`
	Status               models.ApplicationStatus `json:"status"`
	PaymentStatus        models.PaymentStatus     `json:"payment_status"`
	AllDocumentsProvided bool                     `json:"all_documents_provided"`
	ReadyForReview       bool                     `json:"ready_for_review"`
	SubmissionDate       string                   `json:"submission_date"`
	ReviewStartedAt      *string                  `json:"review_started_at"`
	ReviewCompletedAt    *string                  `json:"review_completed_at"`
	FinalApprovalDate    *string                  `json:"final_approval_date"`
	RejectionDate        *string                  `json:"rejection_date"`
	CollectionDate       *string                  `json:"collection_date"`

	// Architect information
	ArchitectFullName    *string `json:"architect_full_name"`
	ArchitectEmail       *string `json:"architect_email"`
	ArchitectPhoneNumber *string `json:"architect_phone_number"`

	// Financial information
	PlanArea        *string `json:"plan_area"`
	DevelopmentLevy *string `json:"development_levy"`
	VATAmount       *string `json:"vat_amount"`
	TotalCost       *string `json:"total_cost"`
	EstimatedCost   *string `json:"estimated_cost"`

	// Document flags
	ProcessedReceiptProvided                 bool `json:"processed_receipt_provided"`
	InitialPlanProvided                      bool `json:"initial_plan_provided"`
	ProcessedTPD1FormProvided                bool `json:"processed_tpd1_form_provided"`
	ProcessedQuotationProvided               bool `json:"processed_quotation_provided"`
	StructuralEngineeringCertificateProvided bool `json:"structural_engineering_certificate_provided"`
	RingBeamCertificateProvided              bool `json:"ring_beam_certificate_provided"`

	// Core relationships
	Applicant     *EnhancedApplicantSummary `json:"applicant"`
	Tariff        *EnhancedTariffSummary    `json:"tariff"`
	VATRate       *VATRateSummary           `json:"vat_rate"`
	ApprovalGroup *EnhancedApprovalGroup    `json:"approval_group"`

	// Assignment and decisions
	GroupAssignments []*EnhancedGroupAssignment `json:"group_assignments"`
	FinalApproverID  *uuid.UUID                 `json:"final_approver_id"`

	// Issues and comments
	Issues   []*EnhancedIssueSummary   `json:"issues"`
	Comments []*EnhancedCommentSummary `json:"comments"`

	// Documents
	ApplicationDocuments []*EnhancedApplicationDocument `json:"application_documents"`

	// Payment
	Payment *PaymentSummary `json:"payment"`

	// Audit
	CreatedBy string  `json:"created_by"`
	UpdatedBy *string `json:"updated_by"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// Enhanced applicant summary
type EnhancedApplicantSummary struct {
	ID             uuid.UUID `json:"id"`
	ApplicantType  string    `json:"applicant_type"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	FullName       string    `json:"full_name"`
	Email          string    `json:"email"`
	PhoneNumber    string    `json:"phone_number"`
	WhatsAppNumber *string   `json:"whatsapp_number"`
	IDNumber       string    `json:"id_number"`
	PostalAddress  string    `json:"postal_address"`
	City           string    `json:"city"`
	Status         string    `json:"status"`
	Debtor         bool      `json:"debtor"`
}

// Enhanced tariff summary
type EnhancedTariffSummary struct {
	ID                     uuid.UUID `json:"id"`
	Currency               string    `json:"currency"`
	PricePerSquareMeter    string    `json:"price_per_square_meter"`
	PermitFee              string    `json:"permit_fee"`
	InspectionFee          string    `json:"inspection_fee"`
	DevelopmentLevyPercent string    `json:"development_levy_percent"`
	DevelopmentCategory    string    `json:"development_category"`
}

// VAT rate summary
type VATRateSummary struct {
	ID   uuid.UUID `json:"id"`
	Rate string    `json:"rate"`
}

// Enhanced approval group
type EnhancedApprovalGroup struct {
	ID                   uuid.UUID                `json:"id"`
	Name                 string                   `json:"name"`
	Description          string                   `json:"description"`
	Type                 models.ApprovalGroupType `json:"type"`
	IsActive             bool                     `json:"is_active"`
	RequiresAllApprovals bool                     `json:"requires_all_approvals"`
	MinimumApprovals     int                      `json:"minimum_approvals"`
	AutoAssignBackups    bool                     `json:"auto_assign_backups"`
	Members              []*EnhancedGroupMember   `json:"members"`
}

// Enhanced group member
type EnhancedGroupMember struct {
	ID                 uuid.UUID                 `json:"id"`
	UserID             uuid.UUID                 `json:"user_id"`
	FirstName          string                    `json:"first_name"`
	LastName           string                    `json:"last_name"`
	Email              string                    `json:"email"`
	Phone              string                    `json:"phone"`
	Role               models.MemberRole         `json:"role"`
	IsActive           bool                      `json:"is_active"`
	CanRaiseIssues     bool                      `json:"can_raise_issues"`
	CanApprove         bool                      `json:"can_approve"`
	CanReject          bool                      `json:"can_reject"`
	IsFinalApprover    bool                      `json:"is_final_approver"`
	AvailabilityStatus models.AvailabilityStatus `json:"availability_status"`
	Department         string                    `json:"department"`
	RoleName           string                    `json:"role_name"` // From user's role
}

// Enhanced group assignment
type EnhancedGroupAssignment struct {
	ID                      uuid.UUID           `json:"id"`
	IsActive                bool                `json:"is_active"`
	AssignedAt              string              `json:"assigned_at"`
	CompletedAt             *string             `json:"completed_at"`
	TotalMembers            int                 `json:"total_members"`
	AvailableMembers        int                 `json:"available_members"`
	ApprovedCount           int                 `json:"approved_count"`
	RejectedCount           int                 `json:"rejected_count"`
	PendingCount            int                 `json:"pending_count"`
	IssuesRaised            int                 `json:"issues_raised"`
	IssuesResolved          int                 `json:"issues_resolved"`
	ReadyForFinalApproval   bool                `json:"ready_for_final_approval"`
	FinalApproverAssignedAt *string             `json:"final_approver_assigned_at"`
	FinalDecisionAt         *string             `json:"final_decision_at"`
	UsedBackupMembers       bool                `json:"used_backup_members"`
	Decisions               []*EnhancedDecision `json:"decisions"`
}

// Enhanced decision
type EnhancedDecision struct {
	ID                      uuid.UUID                   `json:"id"`
	UserID                  uuid.UUID                   `json:"user_id"`
	MemberID                uuid.UUID                   `json:"member_id"`
	FirstName               string                      `json:"first_name"`
	LastName                string                      `json:"last_name"`
	Email                   string                      `json:"email"`
	Status                  models.MemberDecisionStatus `json:"status"`
	Role                    models.MemberRole           `json:"role"`
	DecidedAt               *string                     `json:"decided_at"`
	AssignedAs              models.MemberRole           `json:"assigned_as"`
	IsFinalApproverDecision bool                        `json:"is_final_approver_decision"`
	WasAvailable            bool                        `json:"was_available"`
}

// Enhanced issue summary
type EnhancedIssueSummary struct {
	ID             uuid.UUID                  `json:"id"`
	Title          string                     `json:"title"`
	Description    string                     `json:"description"`
	Priority       string                     `json:"priority"`
	Category       *string                    `json:"category"`
	IsResolved     bool                       `json:"is_resolved"`
	ResolvedAt     *string                    `json:"resolved_at"`
	AssignmentType models.IssueAssignmentType `json:"assignment_type"`
	CreatedAt      string                     `json:"created_at"`
	RaisedByUser   *UserSummary               `json:"raised_by_user"`
	AssignedToUser *UserSummary               `json:"assigned_to_user,omitempty"`
	ChatThreadID   *uuid.UUID                 `json:"chat_thread_id"`
}

// Enhanced comment summary
type EnhancedCommentSummary struct {
	ID          uuid.UUID          `json:"id"`
	CommentType models.CommentType `json:"comment_type"`
	Content     string             `json:"content"`
	CreatedAt   string             `json:"created_at"`
	User        *UserSummary       `json:"user"`
	DecisionID  *uuid.UUID         `json:"decision_id,omitempty"`
	IssueID     *uuid.UUID         `json:"issue_id,omitempty"`
}

// Enhanced application document
type EnhancedApplicationDocument struct {
	ID        uuid.UUID `json:"id"`
	FileName  string    `json:"file_name"`
	FileSize  string    `json:"file_size"`
	FileType  string    `json:"file_type"`
	MimeType  string    `json:"mime_type"`
	FilePath  string    `json:"file_path"`
	CreatedAt string    `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// Payment summary
type PaymentSummary struct {
	ID                uuid.UUID `json:"id"`
	TransactionNumber string    `json:"transaction_number"`
	Amount            string    `json:"amount"`
	PaymentMethod     string    `json:"payment_method"`
	PaymentStatus     string    `json:"payment_status"`
	ReceiptNumber     string    `json:"receipt_number"`
	PaymentDate       string    `json:"payment_date"`
}

// Enhanced chat thread with pagination support
type EnhancedChatThread struct {
	ID           uuid.UUID                 `json:"id"`
	Title        string                    `json:"title"`
	ThreadType   models.ChatThreadType     `json:"thread_type"`
	Description  *string                   `json:"description"`
	IsActive     bool                      `json:"is_active"`
	IsResolved   bool                      `json:"is_resolved"`
	CreatedAt    string                    `json:"created_at"`
	ResolvedAt   *string                   `json:"resolved_at"`
	Participants []*ChatParticipantSummary `json:"participants"`
	Messages     []*EnhancedChatMessage    `json:"messages"`
	HasMore      bool                      `json:"has_more"`    // For pagination
	TotalCount   int                       `json:"total_count"` // Total messages count
}

type MessageSummary struct {
    ID        uuid.UUID `json:"id"`
    Content   string    `json:"content"`
    Sender    *UserSummary `json:"sender"`
    CreatedAt string    `json:"created_at"`
}

// Enhanced chat message with attachments
type EnhancedChatMessage struct {
    ID          uuid.UUID              `json:"id"`
    Content     string                 `json:"content"`
    MessageType models.ChatMessageType `json:"message_type"`
    Status      models.MessageStatus   `json:"status"`
    IsEdited    bool                   `json:"is_edited"`
    EditedAt    *string                `json:"edited_at,omitempty"`
    IsDeleted   bool                   `json:"is_deleted"`
    CreatedAt   string                 `json:"created_at"`
    Sender      *UserSummary           `json:"sender"`
    ParentID    *uuid.UUID             `json:"parent_id,omitempty"`
    Parent      *MessageSummary        `json:"parent,omitempty"` // NEW: For reply threads
    Attachments []*ChatAttachmentSummary `json:"attachments,omitempty"`
	ReadCount  int  `json:"read_count,omitempty"`
	StarCount  int  `json:"star_count,omitempty"`
	IsStarred  bool `json:"is_starred,omitempty"` 
}

// Chat attachment summary
type ChatAttachmentSummary struct {
	ID        uuid.UUID `json:"id"`
	FileName  string    `json:"file_name"`
	FileSize  string    `json:"file_size"`
	FileType  string    `json:"file_type"`
	MimeType  string    `json:"mime_type"`
	FilePath  string    `json:"file_path"`
	CreatedAt string    `json:"created_at"`
}

// User summary (reusable)
type UserSummary struct {
	ID         uuid.UUID `json:"id"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Email      string    `json:"email,omitempty"`
	Phone      string    `json:"phone,omitempty"`
	Department string    `json:"department,omitempty"`
	RoleName   string    `json:"role_name,omitempty"`
}

// repositories/application_repository.go (continued)

func (r *applicationRepository) GetEnhancedApplicationApprovalData(applicationID string) (*ApplicationApprovalData, error) {
	var application models.Application

	// Step 1: Get application with all necessary preloads
	if err := r.db.
		Preload("Applicant").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Preload("ApprovalGroup").
		Preload("GroupAssignments", "is_active = ?", true).
		Preload("GroupAssignments.Decisions").
		Preload("GroupAssignments.Decisions.Member").
		Preload("GroupAssignments.Decisions.User").
		Preload("GroupAssignments.Decisions.User.Role").
		Preload("GroupAssignments.Decisions.User.Department").
		Preload("Issues").
		Preload("Issues.RaisedByUser").
		Preload("Issues.RaisedByUser.Role").
		Preload("Issues.RaisedByUser.Department").
		Preload("Issues.AssignedToUser").
		Preload("Issues.AssignedToUser.Role").
		Preload("Issues.AssignedToUser.Department").
		Preload("Comments").
		Preload("Comments.User").
		Preload("Comments.User.Role").
		Preload("Comments.User.Department").
		Preload("ApplicationDocuments.Document").
		Preload("Payment").
		Preload("FinalApprover").
		Where("id = ?", applicationID).
		First(&application).Error; err != nil {
		return nil, err
	}

	// Step 2: Load approval group members
	var groupMembers []models.ApprovalGroupMember
	if application.ApprovalGroup.ID != uuid.Nil {
		if err := r.db.
			Preload("User").
			Preload("User.Role").
			Preload("User.Department").
			Where("approval_group_id = ? AND is_active = ?", application.ApprovalGroup.ID, true).
			Find(&groupMembers).Error; err != nil {
			return nil, err
		}
	}

	// Step 3: Load chat threads with last 10 messages and attachments
	var chatThreads []models.ChatThread
	if err := r.db.
		Preload("Participants.User").
		Preload("Participants.User.Role").
		Preload("Participants.User.Department").
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.
				Preload("Sender").
				Preload("Sender.Role").
				Preload("Sender.Department").
				Preload("Attachments.Document").
				Order("created_at DESC").
				Limit(10) // Last 10 messages initially
		}).
		Where("application_id = ? AND is_active = ?", applicationID, true).
		Find(&chatThreads).Error; err != nil {
		return nil, err
	}

	// Step 4: Get total message count for each thread (for pagination)
	threadMessageCounts := make(map[uuid.UUID]int)
	for _, thread := range chatThreads {
		var count int64
		if err := r.db.Model(&models.ChatMessage{}).
			Where("thread_id = ? AND is_deleted = ?", thread.ID, false).
			Count(&count).Error; err == nil {
			threadMessageCounts[thread.ID] = int(count)
		}
	}

	// Step 5: Build the enhanced response
	response := &ApplicationApprovalData{
		Application:      r.buildEnhancedApplicationView(&application, groupMembers, threadMessageCounts),
		ApprovalProgress: r.calculateEnhancedApprovalProgress(&application, groupMembers),
		UnresolvedIssues: r.countUnresolvedIssues(application.Issues),
		CanTakeAction:    r.canTakeAction(&application),
		Workflow:         r.getEnhancedWorkflowStatus(&application, groupMembers),
		ChatThreads:      r.buildEnhancedChatThreads(chatThreads, threadMessageCounts),
	}

	return response, nil
}

// Build enhanced application view
func (r *applicationRepository) buildEnhancedApplicationView(
	app *models.Application,
	members []models.ApprovalGroupMember,
	threadMessageCounts map[uuid.UUID]int,
) *EnhancedApplicationView {
	view := &EnhancedApplicationView{
		ID:                   app.ID,
		PlanNumber:           app.PlanNumber,
		PermitNumber:         app.PermitNumber,
		Status:               app.Status,
		PaymentStatus:        app.PaymentStatus,
		AllDocumentsProvided: app.AllDocumentsProvided,
		ReadyForReview:       app.ReadyForReview,
		SubmissionDate:       app.SubmissionDate.Format(time.RFC3339),

		// Architect info
		ArchitectFullName:    app.ArchitectFullName,
		ArchitectEmail:       app.ArchitectEmail,
		ArchitectPhoneNumber: app.ArchitectPhoneNumber,

		// Financial info
		PlanArea:        utils.DecimalToString(app.PlanArea),
		DevelopmentLevy: utils.DecimalToString(app.DevelopmentLevy),
		VATAmount:       utils.DecimalToString(app.VATAmount),
		TotalCost:       utils.DecimalToString(app.TotalCost),
		EstimatedCost:   utils.DecimalToString(app.EstimatedCost),

		// Document flags
		ProcessedReceiptProvided:                 app.ProcessedReceiptProvided,
		InitialPlanProvided:                      app.InitialPlanProvided,
		ProcessedTPD1FormProvided:                app.ProcessedTPD1FormProvided,
		ProcessedQuotationProvided:               app.ProcessedQuotationProvided,
		StructuralEngineeringCertificateProvided: app.StructuralEngineeringCertificateProvided,
		RingBeamCertificateProvided:              app.RingBeamCertificateProvided,

		// Core relationships
		Applicant:     r.buildEnhancedApplicantSummary(&app.Applicant),
		Tariff:        r.buildEnhancedTariffSummary(app.Tariff),
		VATRate:       r.buildVATRateSummary(app.VATRate),
		ApprovalGroup: r.buildEnhancedApprovalGroup(app.ApprovalGroup, members),

		// Assignments and decisions
		GroupAssignments: r.buildEnhancedGroupAssignments(app.GroupAssignments),
		FinalApproverID:  app.FinalApproverID,

		// Issues and comments
		Issues:   r.buildEnhancedIssueSummaries(app.Issues, threadMessageCounts),
		Comments: r.buildEnhancedCommentSummaries(app.Comments),

		// Documents
		ApplicationDocuments: r.buildEnhancedApplicationDocuments(app.ApplicationDocuments),

		// Payment
		Payment: r.buildPaymentSummary(&app.Payment),

		// Audit
		CreatedBy: app.CreatedBy,
		UpdatedBy: app.UpdatedBy,
		CreatedAt: app.CreatedAt.Format(time.RFC3339),
		UpdatedAt: app.UpdatedAt.Format(time.RFC3339),
	}

	// Add timestamp fields
	view.ReviewStartedAt = utils.FormatTimePointer(app.ReviewStartedAt)
	view.ReviewCompletedAt = utils.FormatTimePointer(app.ReviewCompletedAt)
	view.FinalApprovalDate = utils.FormatTimePointer(app.FinalApprovalDate)
	view.RejectionDate = utils.FormatTimePointer(app.RejectionDate)
	view.CollectionDate = utils.FormatTimePointer(app.CollectionDate)

	return view
}

// Build enhanced applicant summary
func (r *applicationRepository) buildEnhancedApplicantSummary(applicant *models.Applicant) *EnhancedApplicantSummary {
	if applicant == nil {
		return nil
	}
	return &EnhancedApplicantSummary{
		ID:             applicant.ID,
		ApplicantType:  string(applicant.ApplicantType),
		FirstName:      *applicant.FirstName,
		LastName:       *applicant.LastName,
		FullName:       applicant.FullName,
		Email:          applicant.Email,
		PhoneNumber:    applicant.PhoneNumber,
		WhatsAppNumber: applicant.WhatsAppNumber,
		IDNumber:       utils.DerefString(applicant.IdNumber),
		PostalAddress:  *applicant.PostalAddress,
		City:           *applicant.City,
		Status:         string(applicant.Status),
		Debtor:         applicant.Debtor,
	}
}

// Build enhanced tariff summary
func (r *applicationRepository) buildEnhancedTariffSummary(tariff *models.Tariff) *EnhancedTariffSummary {
	if tariff == nil {
		return nil
	}

	devCategory := ""
	if &tariff.DevelopmentCategory != nil {
		devCategory = tariff.DevelopmentCategory.Name
	}

	return &EnhancedTariffSummary{
		ID:                     tariff.ID,
		Currency:               tariff.Currency,
		PricePerSquareMeter:    tariff.PricePerSquareMeter.String(),
		PermitFee:              tariff.PermitFee.String(),
		InspectionFee:          tariff.InspectionFee.String(),
		DevelopmentLevyPercent: tariff.DevelopmentLevyPercent.String(),
		DevelopmentCategory:    devCategory,
	}
}

// Build VAT rate summary
func (r *applicationRepository) buildVATRateSummary(vatRate *models.VATRate) *VATRateSummary {
	if vatRate == nil {
		return nil
	}
	return &VATRateSummary{
		ID:   vatRate.ID,
		Rate: vatRate.Rate.String(),
	}
}

// Build enhanced approval group
func (r *applicationRepository) buildEnhancedApprovalGroup(
	group *models.ApprovalGroup,
	members []models.ApprovalGroupMember,
) *EnhancedApprovalGroup {
	if group == nil {
		return nil
	}

	memberSummaries := make([]*EnhancedGroupMember, len(members))
	for i, member := range members {
		department := ""
		roleName := ""
		if member.User.Department != nil {
			department = member.User.Department.Name
		}
		if &member.User.Role != nil {
			roleName = member.User.Role.Name
		}

		memberSummaries[i] = &EnhancedGroupMember{
			ID:                 member.ID,
			UserID:             member.UserID,
			FirstName:          member.User.FirstName,
			LastName:           member.User.LastName,
			Email:              member.User.Email,
			Phone:              member.User.Phone,
			Role:               member.Role,
			IsActive:           member.IsActive,
			CanRaiseIssues:     member.CanRaiseIssues,
			CanApprove:         member.CanApprove,
			CanReject:          member.CanReject,
			IsFinalApprover:    member.IsFinalApprover,
			AvailabilityStatus: member.AvailabilityStatus,
			Department:         department,
			RoleName:           roleName,
		}
	}

	return &EnhancedApprovalGroup{
		ID:                   group.ID,
		Name:                 group.Name,
		Description:          utils.DerefString(group.Description),
		Type:                 group.Type,
		IsActive:             group.IsActive,
		RequiresAllApprovals: group.RequiresAllApprovals,
		MinimumApprovals:     group.MinimumApprovals,
		AutoAssignBackups:    group.AutoAssignBackups,
		Members:              memberSummaries,
	}
}

// Build enhanced group assignments
func (r *applicationRepository) buildEnhancedGroupAssignments(assignments []models.ApplicationGroupAssignment) []*EnhancedGroupAssignment {
	result := make([]*EnhancedGroupAssignment, len(assignments))
	for i, assignment := range assignments {
		decisionSummaries := make([]*EnhancedDecision, len(assignment.Decisions))
		for j, decision := range assignment.Decisions {
			decisionSummaries[j] = &EnhancedDecision{
				ID:                      decision.ID,
				UserID:                  decision.UserID,
				MemberID:                decision.MemberID,
				FirstName:               decision.User.FirstName,
				LastName:                decision.User.LastName,
				Email:                   decision.User.Email,
				Status:                  decision.Status,
				Role:                    decision.AssignedAs,
				DecidedAt:               utils.FormatTimePointer(decision.DecidedAt),
				AssignedAs:              decision.AssignedAs,
				IsFinalApproverDecision: decision.IsFinalApproverDecision,
				WasAvailable:            decision.WasAvailable,
			}
		}

		result[i] = &EnhancedGroupAssignment{
			ID:                      assignment.ID,
			IsActive:                assignment.IsActive,
			AssignedAt:              assignment.AssignedAt.Format(time.RFC3339),
			CompletedAt:             utils.FormatTimePointer(assignment.CompletedAt),
			TotalMembers:            assignment.TotalMembers,
			AvailableMembers:        assignment.AvailableMembers,
			ApprovedCount:           assignment.ApprovedCount,
			RejectedCount:           assignment.RejectedCount,
			PendingCount:            assignment.PendingCount,
			IssuesRaised:            assignment.IssuesRaised,
			IssuesResolved:          assignment.IssuesResolved,
			ReadyForFinalApproval:   assignment.ReadyForFinalApproval,
			FinalApproverAssignedAt: utils.FormatTimePointer(assignment.FinalApproverAssignedAt),
			FinalDecisionAt:         utils.FormatTimePointer(assignment.FinalDecisionAt),
			UsedBackupMembers:       assignment.UsedBackupMembers,
			Decisions:               decisionSummaries,
		}
	}
	return result
}

// Build enhanced issue summaries
func (r *applicationRepository) buildEnhancedIssueSummaries(
	issues []models.ApplicationIssue,
	threadMessageCounts map[uuid.UUID]int,
) []*EnhancedIssueSummary {
	result := make([]*EnhancedIssueSummary, len(issues))
	for i, issue := range issues {
		var assignedToUser *UserSummary
		if issue.AssignedToUser != nil {
			assignedToUser = &UserSummary{
				ID:        issue.AssignedToUser.ID,
				FirstName: issue.AssignedToUser.FirstName,
				LastName:  issue.AssignedToUser.LastName,
				Email:     issue.AssignedToUser.Email,
				Department: utils.DerefString(func() *string {
					if issue.AssignedToUser.Department != nil {
						return &issue.AssignedToUser.Department.Name
					}
					return nil
				}()),
				RoleName: utils.DerefString(func() *string {
					if &issue.AssignedToUser.Role != nil {
						return &issue.AssignedToUser.Role.Name
					}
					return nil
				}()),
			}
		}

		result[i] = &EnhancedIssueSummary{
			ID:             issue.ID,
			Title:          issue.Title,
			Description:    issue.Description,
			Priority:       issue.Priority,
			Category:       issue.Category,
			IsResolved:     issue.IsResolved,
			ResolvedAt:     utils.FormatTimePointer(issue.ResolvedAt),
			AssignmentType: issue.AssignmentType,
			CreatedAt:      issue.CreatedAt.Format(time.RFC3339),
			RaisedByUser: &UserSummary{
				ID:        issue.RaisedByUser.ID,
				FirstName: issue.RaisedByUser.FirstName,
				LastName:  issue.RaisedByUser.LastName,
				Email:     issue.RaisedByUser.Email,
				Department: utils.DerefString(func() *string {
					if issue.RaisedByUser.Department != nil {
						return &issue.RaisedByUser.Department.Name
					}
					return nil
				}()),
				RoleName: utils.DerefString(func() *string {
					if &issue.RaisedByUser.Role != nil {
						return &issue.RaisedByUser.Role.Name
					}
					return nil
				}()),
			},
			AssignedToUser: assignedToUser,
			ChatThreadID:   issue.ChatThreadID,
		}
	}
	return result
}

// Build enhanced comment summaries
func (r *applicationRepository) buildEnhancedCommentSummaries(comments []models.Comment) []*EnhancedCommentSummary {
	result := make([]*EnhancedCommentSummary, len(comments))
	for i, comment := range comments {
		result[i] = &EnhancedCommentSummary{
			ID:          comment.ID,
			CommentType: comment.CommentType,
			Content:     comment.Content,
			CreatedAt:   comment.CreatedAt.Format(time.RFC3339),
			User: &UserSummary{
				ID:        comment.User.ID,
				FirstName: comment.User.FirstName,
				LastName:  comment.User.LastName,
				Email:     comment.User.Email,
				Department: utils.DerefString(func() *string {
					if comment.User.Department != nil {
						return &comment.User.Department.Name
					}
					return nil
				}()),
				RoleName: utils.DerefString(func() *string {
					if &comment.User.Role != nil {
						return &comment.User.Role.Name
					}
					return nil
				}()),
			},
			DecisionID: comment.DecisionID,
			IssueID:    comment.IssueID,
		}
	}
	return result
}

// Build enhanced application documents
func (r *applicationRepository) buildEnhancedApplicationDocuments(docs []models.ApplicationDocument) []*EnhancedApplicationDocument {
	result := make([]*EnhancedApplicationDocument, len(docs))
	for i, doc := range docs {
		result[i] = &EnhancedApplicationDocument{
			ID:        doc.Document.ID,
			FileName:  doc.Document.FileName,
			FileSize:  doc.Document.FileSize.String(),
			FileType:  string(doc.Document.DocumentType),
			MimeType:  doc.Document.MimeType,
			FilePath:  doc.Document.FilePath,
			CreatedAt: doc.Document.CreatedAt.Format(time.RFC3339),
			CreatedBy: doc.CreatedBy,
		}
	}
	return result
}

// Build payment summary
func (r *applicationRepository) buildPaymentSummary(payment *models.Payment) *PaymentSummary {
	if payment == nil {
		return nil
	}
	return &PaymentSummary{
		ID:                payment.ID,
		TransactionNumber: payment.TransactionNumber,
		Amount:            payment.Amount.String(),
		PaymentMethod:     string(payment.PaymentMethod),
		PaymentStatus:     string(payment.PaymentStatus),
		ReceiptNumber:     payment.ReceiptNumber,
		PaymentDate:       payment.PaymentDate.Format(time.RFC3339),
	}
}

// Build enhanced chat threads
func (r *applicationRepository) buildEnhancedChatThreads(
	threads []models.ChatThread,
	messageCounts map[uuid.UUID]int,
) []*EnhancedChatThread {
	result := make([]*EnhancedChatThread, len(threads))
	for i, thread := range threads {
		// Build participants
		participants := make([]*ChatParticipantSummary, len(thread.Participants))
		for j, participant := range thread.Participants {
			participants[j] = &ChatParticipantSummary{
				ID:        participant.User.ID,
				FirstName: participant.User.FirstName,
				LastName:  participant.User.LastName,
				Email:     participant.User.Email,
				Role:      string(participant.Role),
			}
		}

		// Build messages (reverse to show in chronological order)
		messages := make([]*EnhancedChatMessage, len(thread.Messages))
		for k, message := range thread.Messages {
			// Build attachments
			attachments := make([]*ChatAttachmentSummary, len(message.Attachments))
			for l, attachment := range message.Attachments {
				attachments[l] = &ChatAttachmentSummary{
					ID:        attachment.ID,
					FileName:  attachment.Document.FileName,
					FileSize:  attachment.Document.FileSize.String(),
					FileType:  string(attachment.Document.DocumentType),
					MimeType:  attachment.Document.MimeType,
					FilePath:  attachment.Document.FilePath,
					CreatedAt: attachment.Document.CreatedAt.Format(time.RFC3339),
				}
			}

			messages[len(thread.Messages)-1-k] = &EnhancedChatMessage{
				ID:          message.ID,
				Content:     message.Content,
				MessageType: message.MessageType,
				Status:      message.Status,
				IsEdited:    message.IsEdited,
				EditedAt:    utils.FormatTimePointer(message.EditedAt),
				IsDeleted:   message.IsDeleted,
				CreatedAt:   message.CreatedAt.Format(time.RFC3339),
				Sender: &UserSummary{
					ID:        message.Sender.ID,
					FirstName: message.Sender.FirstName,
					LastName:  message.Sender.LastName,
					Email:     message.Sender.Email,
					Department: utils.DerefString(func() *string {
						if message.Sender.Department != nil {
							return &message.Sender.Department.Name
						}
						return nil
					}()),
					RoleName: utils.DerefString(func() *string {
						if &message.Sender.Role != nil {
							return &message.Sender.Role.Name
						}
						return nil
					}()),
				},
				ParentID:    message.ParentID,
				Attachments: attachments,
			}
		}

		totalCount := messageCounts[thread.ID]
		hasMore := totalCount > len(messages)

		result[i] = &EnhancedChatThread{
			ID:           thread.ID,
			Title:        thread.Title,
			ThreadType:   thread.ThreadType,
			Description:  thread.Description,
			IsActive:     thread.IsActive,
			IsResolved:   thread.IsResolved,
			CreatedAt:    thread.CreatedAt.Format(time.RFC3339),
			ResolvedAt:   utils.FormatTimePointer(thread.ResolvedAt),
			Participants: participants,
			Messages:     messages,
			HasMore:      hasMore,
			TotalCount:   totalCount,
		}
	}
	return result
}

// repositories/application_repository.go

// Count unresolved issues
func (r *applicationRepository) countUnresolvedIssues(issues []models.ApplicationIssue) int {
	count := 0
	for _, issue := range issues {
		if !issue.IsResolved {
			count++
		}
	}
	return count
}

// Check if current user can take action
func (r *applicationRepository) canTakeAction(app *models.Application) bool {
	return app.PaymentStatus == models.PaidPayment &&
		app.AllDocumentsProvided &&
		app.Status == models.UnderReviewApplication
}

// Calculate enhanced approval progress
func (r *applicationRepository) calculateEnhancedApprovalProgress(
	app *models.Application,
	members []models.ApprovalGroupMember,
) int {
	if len(members) == 0 {
		return 0
	}

	// Count only regular members (non-final approvers)
	regularMemberCount := 0
	approvedCount := 0

	for _, member := range members {
		if !member.IsFinalApprover && member.IsActive {
			regularMemberCount++
			// Check if this member has approved in any assignment
			for _, assignment := range app.GroupAssignments {
				for _, decision := range assignment.Decisions {
					if decision.MemberID == member.ID && decision.Status == models.DecisionApproved {
						approvedCount++
						break
					}
				}
			}
		}
	}

	if regularMemberCount == 0 {
		return 0
	}

	return (approvedCount * 100) / regularMemberCount
}

// Get enhanced workflow status
func (r *applicationRepository) getEnhancedWorkflowStatus(
	app *models.Application,
	members []models.ApprovalGroupMember,
) *WorkflowStatus {
	// Count unique departments
	departmentMap := make(map[uuid.UUID]bool)
	approvedDepartments := make(map[uuid.UUID]bool)

	for _, member := range members {
		if member.User.Department != nil && member.User.Department.ID != uuid.Nil {
			departmentMap[member.User.Department.ID] = true

			// Check if this department has approved
			for _, assignment := range app.GroupAssignments {
				for _, decision := range assignment.Decisions {
					if decision.MemberID == member.ID &&
						decision.Status == models.DecisionApproved {
						approvedDepartments[member.User.Department.ID] = true
						break
					}
				}
			}
		}
	}

	totalDepartments := len(departmentMap)
	approvedCount := len(approvedDepartments)

	var progressPercentage int
	if totalDepartments > 0 {
		progressPercentage = (approvedCount * 100) / totalDepartments
	}

	return &WorkflowStatus{
		TotalDepartments:    totalDepartments,
		ApprovedDepartments: approvedCount,
		ProgressPercentage:  progressPercentage,
	}
}

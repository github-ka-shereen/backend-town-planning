package repositories

import (
	"fmt"
	"time"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ApprovalResult and RejectionResult types
type ApprovalResult struct {
	ApplicationStatus     models.ApplicationStatus
	IsFinalApprover       bool
	ReadyForFinalApproval bool
	ApprovedCount         int
	TotalMembers          int
	UnresolvedIssues      int
}

type RejectionResult struct {
	ApplicationStatus models.ApplicationStatus
	IsFinalApprover   bool
}


// UpdateApplication updates an application with the provided fields
func (r *applicationRepository) UpdateApplication(
	tx *gorm.DB,
	applicationID uuid.UUID,
	updates map[string]interface{},
) (*models.Application, error) {
	var application models.Application

	// First, fetch the existing application
	if err := tx.First(&application, "id = ?", applicationID).Error; err != nil {
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// Apply updates
	if err := tx.Model(&application).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update application: %w", err)
	}

	// Reload with relationships
	if err := tx.
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Preload("Documents", "is_active = ?", true).
		Preload("ApprovalGroup").
		Preload("FinalApprover").
		First(&application, "id = ?", applicationID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload application: %w", err)
	}

	return &application, nil
}

// GetApplicationForUpdate fetches application with all necessary relationships for updates
func (r *applicationRepository) GetApplicationForUpdate(applicationID string) (*models.Application, error) {
	var application models.Application

	err := r.db.
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Preload("Documents", "is_active = ?", true).
		Preload("Payment").
		Preload("ApprovalGroup").
		Preload("ApprovalGroup.Members", "is_active = ?", true).
		Preload("ApprovalGroup.Members.User").
		Preload("FinalApprover").
		Preload("Stand").
		Where("id = ?", applicationID).
		First(&application).Error

	if err != nil {
		return nil, err
	}

	return &application, nil
}

// UpdateApplicationStatus updates the application status and related timestamps
func (r *applicationRepository) UpdateApplicationStatus(
	tx *gorm.DB,
	applicationID uuid.UUID,
	status models.ApplicationStatus,
	updatedBy string,
) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_by": updatedBy,
	}

	now := time.Now()

	// Set appropriate timestamp based on status
	switch status {
	case models.UnderReviewApplication:
		updates["review_started_at"] = &now
	case models.ApprovedApplication:
		updates["final_approval_date"] = &now
		updates["review_completed_at"] = &now
	case models.RejectedApplication:
		updates["rejection_date"] = &now
		updates["review_completed_at"] = &now
	case models.CollectedApplication:
		updates["collection_date"] = &now
		updates["is_collected"] = true
	case models.ReadyForCollectionApplication:
		// Just update status, no additional timestamps
	}

	return tx.Model(&models.Application{}).
		Where("id = ?", applicationID).
		Updates(updates).Error
}

// UpdateApplicationArchitect updates architect information
func (r *applicationRepository) UpdateApplicationArchitect(
	tx *gorm.DB,
	applicationID uuid.UUID,
	architectFullName *string,
	architectEmail *string,
	architectPhone *string,
	updatedBy string,
) error {
	updates := map[string]interface{}{
		"updated_by": updatedBy,
	}

	if architectFullName != nil {
		updates["architect_full_name"] = architectFullName
	}
	if architectEmail != nil {
		updates["architect_email"] = architectEmail
	}
	if architectPhone != nil {
		updates["architect_phone_number"] = architectPhone
	}

	return tx.Model(&models.Application{}).
		Where("id = ?", applicationID).
		Updates(updates).Error
}

// RecalculateApplicationCosts recalculates financial fields based on tariff and plan area
func (r *applicationRepository) RecalculateApplicationCosts(
	tx *gorm.DB,
	applicationID uuid.UUID,
	tariffID uuid.UUID,
	vatRateID uuid.UUID,
	planArea decimal.Decimal,
) (*CostCalculation, error) {
	// Fetch tariff
	var tariff models.Tariff
	if err := tx.Preload("DevelopmentCategory").First(&tariff, "id = ?", tariffID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch tariff: %w", err)
	}

	// Fetch VAT rate
	var vatRate models.VATRate
	if err := tx.First(&vatRate, "id = ?", vatRateID).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch VAT rate: %w", err)
	}

	// Calculate costs
	areaCost := planArea.Mul(tariff.PricePerSquareMeter)
	subtotal := areaCost.Add(tariff.PermitFee).Add(tariff.InspectionFee)
	developmentLevy := subtotal.Mul(tariff.DevelopmentLevyPercent).Div(decimal.NewFromInt(100))
	totalBeforeVAT := subtotal.Add(developmentLevy)
	vatAmount := totalBeforeVAT.Mul(vatRate.Rate)
	totalCost := totalBeforeVAT.Add(vatAmount)

	// Update application
	updates := map[string]interface{}{
		"plan_area":        planArea,
		"tariff_id":        tariffID,
		"vat_rate_id":      vatRateID,
		"development_levy": developmentLevy,
		"vat_amount":       vatAmount,
		"total_cost":       totalCost,
	}

	if err := tx.Model(&models.Application{}).
		Where("id = ?", applicationID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update costs: %w", err)
	}

	return &CostCalculation{
		AreaCost:        areaCost,
		PermitFee:       tariff.PermitFee,
		InspectionFee:   tariff.InspectionFee,
		DevelopmentLevy: developmentLevy,
		VATAmount:       vatAmount,
		TotalCost:       totalCost,
	}, nil
}

// MarkApplicationAsCollected marks an application as collected
func (r *applicationRepository) MarkApplicationAsCollected(
	tx *gorm.DB,
	applicationID uuid.UUID,
	collectedBy string,
	collectionDate *time.Time,
) error {
	if collectionDate == nil {
		now := time.Now()
		collectionDate = &now
	}

	updates := map[string]interface{}{
		"is_collected":    true,
		"collected_by":    collectedBy,
		"collection_date": collectionDate,
		"status":          models.CollectedApplication,
	}

	return tx.Model(&models.Application{}).
		Where("id = ?", applicationID).
		Updates(updates).Error
}

// UpdateApplicationDocumentFlags updates document verification flags
func (r *applicationRepository) UpdateApplicationDocumentFlags(
	tx *gorm.DB,
	applicationID uuid.UUID,
	documentFlags map[string]bool,
	updatedBy string,
) error {
	updates := map[string]interface{}{
		"updated_by": updatedBy,
	}

	for key, value := range documentFlags {
		updates[key] = value
	}

	// Check if all mandatory documents are provided
	application := &models.Application{}
	if err := tx.First(application, "id = ?", applicationID).Error; err != nil {
		return err
	}

	// Helper to get bool value
	getBoolValue := func(field string, current bool) bool {
		if val, exists := updates[field]; exists {
			if boolVal, ok := val.(bool); ok {
				return boolVal
			}
		}
		return current
	}

	processedReceipt := getBoolValue("processed_receipt_provided", application.ProcessedReceiptProvided)
	initialPlan := getBoolValue("initial_plan_provided", application.InitialPlanProvided)
	tpd1Form := getBoolValue("processed_tpd1_form_provided", application.ProcessedTPD1FormProvided)
	quotation := getBoolValue("processed_quotation_provided", application.ProcessedQuotationProvided)

	allDocsProvided := processedReceipt && initialPlan && tpd1Form && quotation
	updates["all_documents_provided"] = allDocsProvided

	if allDocsProvided {
		now := time.Now()
		updates["documents_completed_at"] = &now
	}

	// Check if ready for review (payment + docs)
	if application.PaymentStatus == models.PaidPayment && allDocsProvided {
		updates["ready_for_review"] = true
	}

	return tx.Model(&models.Application{}).
		Where("id = ?", applicationID).
		Updates(updates).Error
}

// ValidateApplicationForUpdate checks if application can be updated
func (r *applicationRepository) ValidateApplicationForUpdate(applicationID uuid.UUID) error {
	var application models.Application

	if err := r.db.First(&application, "id = ?", applicationID).Error; err != nil {
		return fmt.Errorf("application not found: %w", err)
	}

	// Check if application is in a state that allows updates
	restrictedStatuses := []models.ApplicationStatus{
		models.CollectedApplication,
		models.ExpiredApplication,
	}

	for _, status := range restrictedStatuses {
		if application.Status == status {
			return fmt.Errorf("cannot update application in %s status", status)
		}
	}

	return nil
}

// GetApplicationsByStatus fetches applications by status with pagination
func (r *applicationRepository) GetApplicationsByStatus(
	status models.ApplicationStatus,
	limit, offset int,
) ([]models.Application, int64, error) {
	var applications []models.Application
	var total int64

	query := r.db.Model(&models.Application{}).
		Preload("Applicant").
		Preload("Tariff").
		Preload("Tariff.DevelopmentCategory").
		Preload("VATRate").
		Where("status = ?", status)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated results
	if err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&applications).Error; err != nil {
		return nil, 0, err
	}

	return applications, total, nil
}

// CostCalculation holds the result of cost calculations
type CostCalculation struct {
	AreaCost        decimal.Decimal
	PermitFee       decimal.Decimal
	InspectionFee   decimal.Decimal
	DevelopmentLevy decimal.Decimal
	VATAmount       decimal.Decimal
	TotalCost       decimal.Decimal
}

package services

import (
	"fmt"
	"strings"

	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Define a minimal interface for what the validation needs
type StandTypeGetter interface {
	GetStandTypeByName(name string) (*models.StandType, error)
	GetProjectByProjectNumber(projectNumber string) (*models.Project, error)
}

// ValidateStandRow now accepts the interface instead of the full controller
func ValidateStandRow(row []string, rowIndex int, standRepo StandTypeGetter, createdBy string) (models.Stand, error) {
	if len(row) < 6 {
		return models.Stand{}, fmt.Errorf("row %d has insufficient columns", rowIndex+1)
	}

	standNumber := row[0]
	taxExclusiveStandPriceStr := row[1]
	standSizeStr := row[2]
	standCurrency := row[3]
	standTypeName := row[4]
	projectNumber := row[5]

	var validationErrors []string

	if standNumber == "" {
		validationErrors = append(validationErrors, "Stand Number is empty")
	}

	taxExclusiveStandPrice, err := decimal.NewFromString(taxExclusiveStandPriceStr)
	if err != nil || taxExclusiveStandPrice.IsZero() || taxExclusiveStandPrice.IsNegative() {
		validationErrors = append(validationErrors, "Invalid or non-positive Tax Exclusive Stand Price")
	}

	standSize, err := decimal.NewFromString(standSizeStr)
	if err != nil || standSize.IsZero() || standSize.IsNegative() {
		validationErrors = append(validationErrors, "Invalid or non-positive Stand Size")
	}

	if standCurrency != string(models.USDStandCurrency) && standCurrency != string(models.ZWLStandCurrency) {
		validationErrors = append(validationErrors, "Invalid Stand Currency, must be USD or ZWL")
	}

	var standTypeID *uuid.UUID
	if standTypeName != "" {
		standType, err := standRepo.GetStandTypeByName(standTypeName)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("Invalid Stand Type: %s", err.Error()))
		} else {
			standTypeID = &standType.ID
		}
	} else {
		validationErrors = append(validationErrors, "Stand Type is required")
	}

	if len(validationErrors) > 0 {
		return models.Stand{}, fmt.Errorf("row %d: validation failed: %s", rowIndex+1, strings.Join(validationErrors, ", "))
	}

	project, err := standRepo.GetProjectByProjectNumber(projectNumber)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.Stand{}, &ProjectNotFoundError{
				BulkUploadError: models.BulkUploadErrorStands{
					ID:                     uuid.New(),
					StandNumber:            standNumber,
					TaxExclusiveStandPrice: taxExclusiveStandPrice,
					StandSize:              standSize,
					StandCurrency:          models.StandCurrency(standCurrency),
					StandTypeName:          standTypeName,
					ProjectNumber:          projectNumber,
					Reason:                 err.Error(),
					CreatedBy:              createdBy,
					ErrorType:              models.MissingDataErrorType,
					AddedVia:               models.BulkAddedViaType,
				},
			}
		}
		return models.Stand{}, fmt.Errorf("row %d: unexpected error fetching project: %w", rowIndex+1, err)
	}

	if project == nil {
		return models.Stand{}, fmt.Errorf("row %d: project not found for project number %s", rowIndex+1, projectNumber)
	}

	stand := models.Stand{
		StandNumber:            standNumber,
		TaxExclusiveStandPrice: taxExclusiveStandPrice,
		StandSize:              standSize,
		StandCurrency:          models.StandCurrency(standCurrency),
		StandTypeID:            standTypeID,
		ProjectID:              &project.ID,
	}

	return stand, nil
}

// The rest of your service functions remain the same...
type ProjectNotFoundError struct {
	BulkUploadError models.BulkUploadErrorStands
}

func (e *ProjectNotFoundError) Error() string {
	return e.BulkUploadError.Reason
}

func ValidateStand(stand *models.Stand) string {
	if stand.ProjectID == nil || *stand.ProjectID == uuid.Nil {
		return "Project ID is required"
	}
	if stand.TaxExclusiveStandPrice.IsZero() || stand.TaxExclusiveStandPrice.IsNegative() {
		return "Tax exclusive stand price must be a positive value"
	}
	if stand.StandSize.IsZero() {
		return "Stand size is required"
	}
	if stand.StandCurrency != models.USDStandCurrency && stand.StandCurrency != models.ZWLStandCurrency {
		return "Invalid stand currency, must be USD or ZWL"
	}
	if stand.StandTypeID == nil || *stand.StandTypeID == uuid.Nil {
		return "Stand type is required"
	}
	return ""
}

func IsValidStatus(status models.Status) bool {
	switch status {
	case models.UnallocatedStatus, models.SwappedStatus, models.DonatedStatus,
		models.ReservedStatus, models.FullyPaidStatus, models.PrePlanActivationStatus, models.OngoingPaymentStatus:
		return true
	default:
		return false
	}
}

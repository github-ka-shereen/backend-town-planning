// controllers/application_specific_updates.go
package controllers

import (
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// UpdateApplicationStatusRequest for status-only updates
type UpdateApplicationStatusRequest struct {
	Status models.ApplicationStatus `json:"status" validate:"required"`
}

// UpdateApplicationStatusController updates only the application status
func (ac *ApplicationController) UpdateApplicationStatusController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Get authenticated user
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	// Parse request
	var req UpdateApplicationStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Parse application ID
	appUUID, err := uuid.Parse(applicationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid application ID",
			"error":   "invalid_uuid",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update status
	if err := ac.ApplicationRepo.UpdateApplicationStatus(
		tx,
		appUUID,
		req.Status,
		payload.UserID.String(),
	); err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to update application status",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update application status",
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit transaction",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Application status updated successfully",
		"data": fiber.Map{
			"application_id": applicationID,
			"new_status":     req.Status,
		},
	})
}

// UpdateApplicationArchitectRequest for architect information updates
type UpdateApplicationArchitectRequest struct {
	ArchitectFullName    *string `json:"architect_full_name"`
	ArchitectEmail       *string `json:"architect_email"`
	ArchitectPhoneNumber *string `json:"architect_phone_number"`
}

// UpdateApplicationArchitectController updates architect information
func (ac *ApplicationController) UpdateApplicationArchitectController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Get authenticated user
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	// Parse request
	var req UpdateApplicationArchitectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Parse application ID
	appUUID, err := uuid.Parse(applicationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid application ID",
			"error":   "invalid_uuid",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update architect info
	if err := ac.ApplicationRepo.UpdateApplicationArchitect(
		tx,
		appUUID,
		req.ArchitectFullName,
		req.ArchitectEmail,
		req.ArchitectPhoneNumber,
		payload.UserID.String(),
	); err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to update architect information",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update architect information",
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit transaction",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Architect information updated successfully",
		"data": fiber.Map{
			"application_id": applicationID,
		},
	})
}

// RecalculateApplicationCostsRequest for cost recalculation
type RecalculateApplicationCostsRequest struct {
	TariffID  uuid.UUID       `json:"tariff_id" validate:"required"`
	VATRateID uuid.UUID       `json:"vat_rate_id" validate:"required"`
	PlanArea  decimal.Decimal `json:"plan_area" validate:"required"`
}

// RecalculateApplicationCostsController recalculates application costs
func (ac *ApplicationController) RecalculateApplicationCostsController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Get authenticated user
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	// Parse request
	var req RecalculateApplicationCostsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Parse application ID
	appUUID, err := uuid.Parse(applicationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid application ID",
			"error":   "invalid_uuid",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Recalculate costs
	calculation, err := ac.ApplicationRepo.RecalculateApplicationCosts(
		tx,
		appUUID,
		req.TariffID,
		req.VATRateID,
		req.PlanArea,
	)
	if err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to recalculate costs",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to recalculate application costs",
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit transaction",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Application costs recalculated successfully",
		"data": fiber.Map{
			"application_id":   applicationID,
			"area_cost":        calculation.AreaCost.String(),
			"permit_fee":       calculation.PermitFee.String(),
			"inspection_fee":   calculation.InspectionFee.String(),
			"development_levy": calculation.DevelopmentLevy.String(),
			"vat_amount":       calculation.VATAmount.String(),
			"total_cost":       calculation.TotalCost.String(),
		},
	})
}

// MarkApplicationCollectedRequest for collection marking
type MarkApplicationCollectedRequest struct {
	CollectedBy    string     `json:"collected_by" validate:"required"`
	CollectionDate *time.Time `json:"collection_date"`
}

// MarkApplicationCollectedController marks application as collected
func (ac *ApplicationController) MarkApplicationCollectedController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Get authenticated user
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	// Parse request
	var req MarkApplicationCollectedRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Parse application ID
	appUUID, err := uuid.Parse(applicationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid application ID",
			"error":   "invalid_uuid",
		})
	}

	// Start transaction
	tx := ac.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Mark as collected
	if err := ac.ApplicationRepo.MarkApplicationAsCollected(
		tx,
		appUUID,
		req.CollectedBy,
		req.CollectionDate,
	); err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to mark application as collected",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to mark application as collected",
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit transaction",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Application marked as collected successfully",
		"data": fiber.Map{
			"application_id":  applicationID,
			"collected_by":    req.CollectedBy,
			"collection_date": req.CollectionDate,
		},
	})
}

// UpdateDocumentFlagsRequest for document flag updates
type UpdateDocumentFlagsRequest struct {
	ProcessedReceiptProvided                 *bool `json:"processed_receipt_provided"`
	InitialPlanProvided                      *bool `json:"initial_plan_provided"`
	ProcessedTPD1FormProvided                *bool `json:"processed_tpd1_form_provided"`
	ProcessedQuotationProvided               *bool `json:"processed_quotation_provided"`
	StructuralEngineeringCertificateProvided *bool `json:"structural_engineering_certificate_provided"`
	RingBeamCertificateProvided              *bool `json:"ring_beam_certificate_provided"`
}

// UpdateDocumentFlagsController updates document verification flags
func (ac *ApplicationController) UpdateDocumentFlagsController(c *fiber.Ctx) error {
	applicationID := c.Params("id")

	// Get authenticated user
	payload, ok := c.Locals("user").(*token.Payload)
	if !ok || payload == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not authenticated",
		})
	}

	// Parse request
	var req UpdateDocumentFlagsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Parse application ID
	appUUID, err := uuid.Parse(applicationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid application ID",
			"error":   "invalid_uuid",
		})
	}

	// Build document flags map
	documentFlags := make(map[string]bool)
	if req.ProcessedReceiptProvided != nil {
		documentFlags["processed_receipt_provided"] = *req.ProcessedReceiptProvided
	}
	if req.InitialPlanProvided != nil {
		documentFlags["initial_plan_provided"] = *req.InitialPlanProvided
	}
	if req.ProcessedTPD1FormProvided != nil {
		documentFlags["processed_tpd1_form_provided"] = *req.ProcessedTPD1FormProvided
	}
	if req.ProcessedQuotationProvided != nil {
		documentFlags["processed_quotation_provided"] = *req.ProcessedQuotationProvided
	}
	if req.StructuralEngineeringCertificateProvided != nil {
		documentFlags["structural_engineering_certificate_provided"] = *req.StructuralEngineeringCertificateProvided
	}
	if req.RingBeamCertificateProvided != nil {
		documentFlags["ring_beam_certificate_provided"] = *req.RingBeamCertificateProvided
	}

	// Start transaction
	tx := ac.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update document flags
	if err := ac.ApplicationRepo.UpdateApplicationDocumentFlags(
		tx,
		appUUID,
		documentFlags,
		payload.UserID.String(),
	); err != nil {
		tx.Rollback()
		config.Logger.Error("Failed to update document flags",
			zap.String("applicationID", applicationID),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update document flags",
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit transaction",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Document flags updated successfully",
		"data": fiber.Map{
			"application_id": applicationID,
			"updated_flags":  documentFlags,
		},
	})
}

// GetApplicationsByStatusController gets applications filtered by status
func (ac *ApplicationController) GetApplicationsByStatusController(c *fiber.Ctx) error {
	status := c.Params("status")

	// Parse pagination
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)

	// Validate status
	validStatus := models.ApplicationStatus(status)

	// Get applications
	applications, total, err := ac.ApplicationRepo.GetApplicationsByStatus(validStatus, limit, offset)
	if err != nil {
		config.Logger.Error("Failed to get applications by status",
			zap.String("status", status),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to retrieve applications",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Applications retrieved successfully",
		"data": fiber.Map{
			"applications": applications,
			"total":        total,
			"limit":        limit,
			"offset":       offset,
		},
	})
}

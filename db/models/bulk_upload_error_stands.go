package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type BulkUploadErrorStands struct {
	ID                     uuid.UUID           `gorm:"type:uuid;primary_key;" json:"id"`
	StandNumber            string              `json:"stand_number"`
	ProjectNumber          string              `json:"project_number"`
	Reason                 string              `json:"reason"`
	TaxExclusiveStandPrice decimal.Decimal     `gorm:"type:decimal(18,8)" json:"tax_exclusive_stand_price"` // Added: Price before VAT
	VATAmount              decimal.Decimal     `gorm:"type:decimal(18,8)" json:"vat_amount"`                // Added: Calculated VAT amount
	StandCost              decimal.Decimal     `json:"stand_cost"`                                          // This remains the total cost including VAT
	StandSize              decimal.Decimal     `json:"stand_size"`
	StandCurrency          StandCurrency       `json:"stand_currency"` // USD or ZWL
	StandTypeName          string              `json:"stand_type_name"`     // Residential or Commercial
	CreatedBy              string              `json:"created_by"`
	ErrorType              BulkUploadErrorType `json:"error_type"`
	AddedVia               AddedViaType        `json:"added_via"`
	CreatedAt              time.Time           `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time           `gorm:"autoUpdateTime" json:"updated_at"`
}

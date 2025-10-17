package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ExchangeRate struct {
	ID           uuid.UUID       `gorm:"type:uuid;primary_key;" json:"id"`
	CurrencyName string          `json:"currency_name"`
	CurrencyCode string          `json:"currency_code"` // Ensure this is always uppercase (USD, ZWL)
	ValuePerUsd  decimal.Decimal `gorm:"type:decimal(18,8)" json:"value_per_usd"`
	ValuePerZwl  decimal.Decimal `gorm:"type:decimal(18,8)" json:"value_per_zwl"`
	CreatedAt    time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    *time.Time       `gorm:"autoCreateTime" json:"updated_at"` // Ignore during creation
	Active       bool            `json:"active"`              // If this rate is the current active one
	Used         bool            `json:"used"`
	CreatedBy    string          `gorm:"not null" json:"created_by"`
	UpdatedBy    *string         `json:"updated_by"`
	ValidFrom    time.Time       `json:"valid_from"` // When this rate starts being valid
	ValidTo      *time.Time      `json:"valid_to"`   // When this rate stops being valid (optional, can be null if active)
}

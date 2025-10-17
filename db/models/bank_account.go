package models

import (
	"time"

	"github.com/google/uuid"
)

type AccountType string

const (
	ClientBankAccountType   AccountType = "CLIENT_ACCOUNT"
	InternalBankAccountType AccountType = "INTERNAL_ACCOUNT"
)

type AccountCurrency string

const (
	USDAccountCurrency AccountCurrency = "USD"
	ZWLAccountCurrency AccountCurrency = "ZWL"
)

// Bank struct representing a bank
type Bank struct {
	ID           uuid.UUID     `gorm:"type:uuid;primary_key;" json:"id"`
	BankName     string        `json:"bank_name"`                              // Name of the bank
	BranchName   string        `json:"branch_name"`                            // Branch name
	SwiftCode    string        `json:"swift_code,omitempty"`                   // Optional SWIFT code
	BankAccounts []BankAccount `gorm:"foreignKey:BankID" json:"bank_accounts"` // Associated bank accounts
	IsActive     bool          `json:"is_active"`                              // Indicates if the bank is active
	CreatedAt    time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy    string        `gorm:"not null" json:"created_by"` // Staff member adding the record
	UpdatedBy    string        `json:"updated_by"`
}

// BankAccount struct representing a bank account
type BankAccount struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primary_key;" json:"id"`
	BankID              uuid.UUID       `gorm:"type:uuid;not null" json:"bank_id"` // Foreign key to Bank
	Bank                Bank            `gorm:"foreignKey:BankID" json:"bank"`    // Associated Bank
	AccountName         string          `json:"account_name"`                      // Descriptive name for the account
	BankAccountCurrency AccountCurrency `json:"bank_account_currency"`             // USD, ZWL
	BankAccountType     AccountType     `json:"bank_account_type"`
	AccountNumber       string          `json:"account_number"` // Bank account number
	IsActive            bool            `json:"is_active"`      // Indicates if the account is active
	CreatedAt           time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy           string          `gorm:"not null" json:"created_by"` // Staff member adding the record
	UpdatedBy           string          `json:"updated_by"`
	DeletedAt           *time.Time      `gorm:"index" json:"deleted_at,omitempty"` // Soft delete timestamp
}

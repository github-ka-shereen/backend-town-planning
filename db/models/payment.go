package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

//
// ENUM DEFINITIONS
//

type PaymentMethod string

const (
	CashPaymentMethod        PaymentMethod = "CASH"
	BankDepositPaymentMethod PaymentMethod = "BANK_DEPOSIT"
	ForexPaymentMethod       PaymentMethod = "FOREX"
)

type PaymentFor string

const (
	PaymentForApplicationFee  PaymentFor = "APPLICATION_FEE"
	PaymentForInspectionFee   PaymentFor = "INSPECTION_FEE"
	PaymentForPermitFee       PaymentFor = "PERMIT_FEE"
	PaymentForDevelopmentLevy PaymentFor = "DEVELOPMENT_LEVY"
)

type TransactionType string

const (
	OrdinaryTransactionType         TransactionType = "ORDINARY"
	CreditAdjustmentTransactionType TransactionType = "CREDIT_ADJUSTMENT"
	DebitAdjustmentTransactionType  TransactionType = "DEBIT_ADJUSTMENT"
)

type PaymentStatus string

const (
	PendingPayment   PaymentStatus = "PENDING"
	PaidPayment      PaymentStatus = "PAID"
	PartialPayment   PaymentStatus = "PARTIAL"
	RefundedPayment  PaymentStatus = "REFUNDED"
	CancelledPayment PaymentStatus = "CANCELLED"
)

//
// PAYMENT MODEL
//

type Payment struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`

	// References (link to Application or Tariff)
	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id,omitempty"`
	TariffID      *uuid.UUID `gorm:"type:uuid;index" json:"tariff_id,omitempty"`

	// Purpose and transaction details
	PaymentFor        PaymentFor      `gorm:"type:varchar(50);not null" json:"payment_for"`
	TransactionNumber string          `gorm:"uniqueIndex;not null" json:"transaction_number"`
	TransactionType   TransactionType `gorm:"type:varchar(30);not null" json:"transaction_type"`

	// Reversal handling
	ReversedForTransactionNumber *string `json:"reversed_for_transaction_number,omitempty"`
	ReversalReason               *string `json:"reversal_reason,omitempty"`
	IsReversal                   bool    `gorm:"default:false" json:"is_reversal"`

	// Currency & amounts
	ReceivedCurrency    *string         `gorm:"type:varchar(10)" json:"received_currency"`       // e.g. USD, ZAR
	Amount              decimal.Decimal `gorm:"type:decimal(18,8)" json:"amount"`                // Amount paid in USD
	ReceivedForexAmount decimal.Decimal `gorm:"type:decimal(18,2)" json:"received_forex_amount"` // Converted to system currency (USD/ZWL)

	// Payment method & bank details
	PaymentMethod PaymentMethod `gorm:"type:varchar(30);not null" json:"payment_method"`
	BankAccountID *uuid.UUID    `gorm:"type:uuid;index" json:"bank_account_id"`
	BankAccount   *BankAccount  `gorm:"foreignKey:BankAccountID;references:ID" json:"bank_account,omitempty"`

	ExchangeRateID *uuid.UUID    `gorm:"type:uuid;index" json:"exchange_rate_id"`
	ExchangeRate   *ExchangeRate `gorm:"foreignKey:ExchangeRateID;references:ID" json:"exchange_rate,omitempty"`

	// Status & identifiers
	PaymentStatus     PaymentStatus `gorm:"type:varchar(20);default:'PENDING'" json:"payment_status"`
	ExternalReference *string       `gorm:"index" json:"external_reference,omitempty"` // e.g. Bank Txn, PayNow ref
	ReceiptNumber     string        `gorm:"uniqueIndex;not null" json:"receipt_number"`
	PaymentDate       time.Time     `gorm:"not null" json:"payment_date"`
	Notes             string        `json:"notes,omitempty"`

	// Audit trail
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedBy string    `gorm:"not null" json:"created_by"`
	UpdatedBy *string   `json:"updated_by"`
}

// Automatically generate UUID and TransactionNumber before saving
func (p *Payment) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	if p.TransactionNumber == "" {
		p.TransactionNumber = fmt.Sprintf("TXN-%s", uuid.NewString()[0:8])
	}

	if p.ReceiptNumber == "" {
		p.ReceiptNumber = fmt.Sprintf("RCT-%s", uuid.NewString()[0:8])
	}

	// Default PaymentDate if missing
	if p.PaymentDate.IsZero() {
		p.PaymentDate = time.Now()
	}

	return
}

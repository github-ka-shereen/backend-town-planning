package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type DocumentType string

const (
	WordDocumentType DocumentType = "WORD_DOCUMENT"
	TextDocumentType DocumentType = "TEXT_DOCUMENT"
	SpreadsheetType  DocumentType = "SPREADSHEET"
	PresentationType DocumentType = "PRESENTATION"
	ImageType        DocumentType = "IMAGE"
	PDFType          DocumentType = "PDF"
)

type DocumentCategoryType string

const (
	StandLegalDocuments                  DocumentCategoryType = "STAND_LEGAL_DOCUMENTS"
	ClientIdentityVerificationDocuments  DocumentCategoryType = "CLIENT_IDENTITY_DOCUMENTS"
	ClientCouncilDocuments               DocumentCategoryType = "CLIENT_COUNCIL_DOCUMENTS"
	PurchaseAgreementDocuments           DocumentCategoryType = "PURCHASE_AGREEMENT_DOCUMENTS"
	CompanyDocuments                     DocumentCategoryType = "COMPANY_DOCUMENTS"
	CommunicationCorrespondenceDocuments DocumentCategoryType = "COMMUNICATION_CORRESPONDENCE"
	StandCoOwnershipDocuments            DocumentCategoryType = "STAND_CO-OWNERSHIP_DOCUMENTS"
	CityCouncilApplicationDocuments      DocumentCategoryType = "CITY_COUNCIL_APPLICATION_DOCUMENTS"

	// Add specific document types
	IDCopyDocuments              DocumentCategoryType = "ID_COPY"
	MarriageCertificateDocuments DocumentCategoryType = "MARRIAGE_CERTIFICATE"
	BirthCertificateDocuments    DocumentCategoryType = "BIRTH_CERTIFICATE"
	PaySlipDocuments             DocumentCategoryType = "PAY_SLIP"
	IncomeProofDocuments         DocumentCategoryType = "INCOME_PROOF"
	ApplicationFormDocuments     DocumentCategoryType = "APPLICATION_FORM"

	OtherDocuments DocumentCategoryType = "OTHER_DOCUMENTS"
)

type Document struct {
	ID               uuid.UUID            `gorm:"type:uuid;primary_key;" json:"id"`
	FileName         string               `json:"file_name"`
	DocumentType     DocumentType         `json:"document_type"`
	FileSize         decimal.Decimal      `json:"file_size"`
	DocumentCategory DocumentCategoryType `gorm:"type:varchar(50)" json:"document_category"`
	FilePath         string               `json:"file_path"`
	CreatedBy        string               `json:"created_by"`

	ClientID *uuid.UUID `gorm:"type:uuid;index" json:"client_id,omitempty"`
	Client   *Client    `gorm:"foreignKey:ClientID;references:ID" json:"client,omitempty"`

	ApplicationID *uuid.UUID `gorm:"type:uuid;index" json:"application_id,omitempty"`
	Application   *Application `gorm:"foreignKey:ApplicationID;references:ID" json:"application,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

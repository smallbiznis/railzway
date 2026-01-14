package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
)

// Railzway official tax codes (v1).
// These codes are ENGINE-CONSTANTS.
// Do NOT rename or repurpose once used in invoices.
const (
	// No tax applied
	TaxCodeNoTax = "NO_TAX"

	// United States
	TaxCodeUSSalesTax = "US_SALES_TAX"

	// European Union
	TaxCodeEUVATStandard = "EU_VAT_STANDARD"

	// Asia-Pacific
	TaxCodeSGGST = "SG_GST"
	TaxCodeJPJCT = "JP_JCT"

	// Canada (placeholder for compound tax)
	TaxCodeCACompound = "CA_COMPOUND"

	// Withholding / reverse tax (placeholder)
	TaxCodeWithholding = "WITHHOLDING"
)

// TaxMode represents how tax is applied to the invoice total.
type TaxMode string

const (
	TaxModeExclusive TaxMode = "exclusive" // subtotal + tax
	TaxModeInclusive TaxMode = "inclusive" // total already includes tax
)

// TaxDefinition is an org-scoped tax policy definition.
// NOTE:
// - code is a stable, engine-facing identifier (immutable once created)
// - name/description are UI-facing and editable
type TaxDefinition struct {
	ID    snowflake.ID `gorm:"primaryKey"`
	OrgID snowflake.ID `gorm:"column:org_id;not null;index"`

	Name    string   `gorm:"type:text;not null"`
	Code    string   `gorm:"type:text;not null"`
	TaxMode TaxMode  `gorm:"column:tax_mode;type:text;not null"`
	Rate    *float64 `gorm:"type:numeric(6,4)"` // fraction (e.g. 0.2000 for 20%), nil if dynamic/placeholder

	Description *string `gorm:"type:text"`

	IsEnabled bool `gorm:"column:is_enabled;not null;default:true"`

	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (TaxDefinition) TableName() string { return "tax_definitions" }

func (t *TaxDefinition) Validate() error {
	if t.Code == "" {
		return ErrInvalidTaxCode
	}
	if t.TaxMode != TaxModeExclusive && t.TaxMode != TaxModeInclusive {
		return ErrInvalidTaxMode
	}
	if t.Rate != nil && *t.Rate < 0 {
		return ErrInvalidTaxRate
	}
	return nil
}

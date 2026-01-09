// Package domain contains persistence models for invoicing.
package domain

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

// InvoiceStatus represents invoice lifecycle states.
type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "DRAFT"
	InvoiceStatusFinalized InvoiceStatus = "FINALIZED"
	InvoiceStatusVoid      InvoiceStatus = "VOID"
)

// Invoice represents a generated invoice.
type Invoice struct {
	ID                snowflake.ID      `gorm:"primaryKey"`
	OrgID             snowflake.ID      `gorm:"not null;index;uniqueIndex:ux_invoice_number_org,priority:1"`
	InvoiceSeq        *int64            `gorm:"uniqueIndex:ux_invoice_number_org,priority:2"`
	InvoiceNumber     string            `gorm:"not null;index;"`
	BillingCycleID    snowflake.ID      `gorm:"not null;index;uniqueIndex:ux_invoice_billing_cycle"`
	SubscriptionID    snowflake.ID      `gorm:"not null;index"`
	CustomerID        snowflake.ID      `gorm:"not null;index"`
	InvoiceTemplateID *snowflake.ID     `gorm:"column:invoice_template_id;index"`
	Status            InvoiceStatus     `gorm:"type:text;not null;default:'DRAFT'"`
	SubtotalAmount    int64             `gorm:"not null;default:0"`
	Currency          string            `gorm:"type:text;not null"`
	PeriodStart       *time.Time        `gorm:""`
	PeriodEnd         *time.Time        `gorm:""`
	IssuedAt          *time.Time        `gorm:""`
	DueAt             *time.Time        `gorm:""`
	PaidAt            *time.Time        `gorm:"column:paid_at"`
	FinalizedAt       *time.Time        `gorm:""`
	VoidedAt          *time.Time        `gorm:""`
	RenderedHTML      *string           `gorm:"column:rendered_html;type:text"`
	RenderedPDFURL    *string           `gorm:"column:rendered_pdf_url;type:text"`
	Metadata          datatypes.JSONMap `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt         time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt         time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (Invoice) TableName() string { return "invoices" }

type InvoiceItemLineType string

const (
	// Subscription / plan / flat fee
	InvoiceItemLineTypeSubscription InvoiceItemLineType = "subscription"

	// Usage-based billing (metered)
	InvoiceItemLineTypeUsage InvoiceItemLineType = "usage"

	// Credit / promotion / signup credit / adjustment (negative amount)
	InvoiceItemLineTypeCredit InvoiceItemLineType = "credit"

	// One-off charge (manual charge, setup fee, migration fee)
	InvoiceItemLineTypeOneOff InvoiceItemLineType = "one_off"

	// Tax line (VAT, GST, sales tax)
	InvoiceItemLineTypeTax InvoiceItemLineType = "tax"
)

func (t InvoiceItemLineType) String() string {
	switch t {
	case InvoiceItemLineTypeSubscription, InvoiceItemLineTypeUsage, InvoiceItemLineTypeCredit, InvoiceItemLineTypeOneOff, InvoiceItemLineTypeTax:
		return string(t)
	default:
		return ""
	}
}

// InvoiceItem represents a line on an invoice.
type InvoiceItem struct {
	ID             snowflake.ID        `gorm:"primaryKey"`
	OrgID          snowflake.ID        `gorm:"not null;index"`
	InvoiceID      snowflake.ID        `gorm:"not null;index"`
	RatingResultID *snowflake.ID       `gorm:"index"`
	LineType       InvoiceItemLineType `gorm:"type:text"`
	Description    string              `gorm:"type:text"`
	Quantity       float64             `gorm:"not null"`
	UnitPrice      int64               `gorm:"not null"`
	Amount         int64               `gorm:"not null"`
	Metadata       datatypes.JSONMap   `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt      time.Time           `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// TableName sets the database table name.
func (InvoiceItem) TableName() string { return "invoice_items" }

type InvoiceSequence struct {
	OrgID      snowflake.ID `gorm:"primaryKey"`
	NextNumber int          `gorm:"not null;default:1"`
	UpdatedAt  time.Time    `gorm:"not null"`
}

func (InvoiceSequence) TableName() string { return "invoice_sequences" }

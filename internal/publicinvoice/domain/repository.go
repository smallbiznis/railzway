package domain

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Repository interface {
	FindInvoiceByToken(ctx context.Context, db *gorm.DB, orgID snowflake.ID, token string) (*InvoiceRecord, error)
	ListInvoiceItems(ctx context.Context, db *gorm.DB, orgID snowflake.ID, invoiceID snowflake.ID) ([]InvoiceItemRecord, error)
	ListPaymentMethods(ctx context.Context, db *gorm.DB, orgID snowflake.ID) ([]PaymentMethodRecord, error)
	UpdateInvoiceMetadata(ctx context.Context, db *gorm.DB, orgID snowflake.ID, invoiceID snowflake.ID, metadata datatypes.JSONMap, updatedAt time.Time) error
	FindInvoiceSettledAmount(ctx context.Context, db *gorm.DB, orgID snowflake.ID, invoiceID snowflake.ID, currency string) (int64, error)
}

type InvoiceRecord struct {
	ID             snowflake.ID      `gorm:"column:id"`
	OrgID          snowflake.ID      `gorm:"column:org_id"`
	OrgName        string            `gorm:"column:org_name"`
	InvoiceNumber  string            `gorm:"column:invoice_number"`
	Status         string            `gorm:"column:status"`
	SubtotalAmount int64             `gorm:"column:subtotal_amount"`
	Currency       string            `gorm:"column:currency"`
	IssuedAt       *time.Time        `gorm:"column:issued_at"`
	DueAt          *time.Time        `gorm:"column:due_at"`
	PaidAt         *time.Time        `gorm:"column:paid_at"`
	CustomerID     snowflake.ID      `gorm:"column:customer_id"`
	CustomerName   string            `gorm:"column:customer_name"`
	CustomerEmail  string            `gorm:"column:customer_email"`
	Metadata       datatypes.JSONMap `gorm:"column:metadata"`
}

type InvoiceItemRecord struct {
	Description string  `gorm:"column:description"`
	Quantity    float64 `gorm:"column:quantity"`
	UnitPrice   int64   `gorm:"column:unit_price"`
	Amount      int64   `gorm:"column:amount"`
	LineType    string  `gorm:"column:line_type"`
}

type PaymentMethodRecord struct {
	Provider    string `gorm:"column:provider"`
	DisplayName string `gorm:"column:display_name"`
}

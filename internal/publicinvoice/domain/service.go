package domain

import (
	"context"
	"errors"

	"github.com/bwmarrin/snowflake"
)

type Service interface {
	GetInvoiceForPublicView(ctx context.Context, orgID snowflake.ID, token string) (*PublicInvoiceResponse, error)
	GetInvoicePublicStatus(ctx context.Context, orgID snowflake.ID, token string) (PublicInvoiceStatus, error)
	CreateOrReusePaymentIntent(ctx context.Context, orgID snowflake.ID, token string) (*PaymentIntentResponse, error)
	ListPaymentMethods(ctx context.Context, orgID snowflake.ID) ([]PublicPaymentMethod, error)
}

type PublicInvoiceStatus string

const (
	PublicInvoiceStatusUnpaid     PublicInvoiceStatus = "unpaid"
	PublicInvoiceStatusProcessing PublicInvoiceStatus = "processing"
	PublicInvoiceStatusPaid       PublicInvoiceStatus = "paid"
	PublicInvoiceStatusFailed     PublicInvoiceStatus = "failed"
)

type PublicInvoiceItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   int64   `json:"unit_price"`
	Amount      int64   `json:"amount"`
	LineType    string  `json:"line_type,omitempty"`
}

type PublicInvoiceView struct {
	OrgID          string              `json:"org_id"`
	OrgName        string              `json:"org_name"`
	InvoiceNumber  string              `json:"invoice_number"`
	InvoiceStatus  string              `json:"invoice_status"`
	IssueDate      string              `json:"issue_date"`
	DueDate        string              `json:"due_date"`
	PaidDate       string              `json:"paid_date,omitempty"`
	PaymentState   string              `json:"payment_state,omitempty"`
	BillToName     string              `json:"bill_to_name"`
	BillToEmail    string              `json:"bill_to_email"`
	Currency       string              `json:"currency"`
	AmountDue      int64               `json:"amount_due"`
	SubtotalAmount int64               `json:"subtotal_amount"`
	TaxAmount      int64               `json:"tax_amount"`
	TotalAmount    int64               `json:"total_amount"`
	Items          []PublicInvoiceItem `json:"items"`
}

type PublicInvoiceResponse struct {
	Status  PublicInvoiceStatus `json:"status"`
	Invoice PublicInvoiceView   `json:"invoice"`
}

type PublicPaymentMethod struct {
	Provider            string `json:"provider"`
	Type                string `json:"type"`
	DisplayName         string `json:"display_name"`
	SupportsInstallment bool   `json:"supports_installment"`
	PublishableKey      string `json:"publishable_key,omitempty"`
}

type PaymentIntentResponse struct {
	ClientSecret string `json:"client_secret"`
}

var (
	ErrInvoiceUnavailable = errors.New("invoice_unavailable")
)

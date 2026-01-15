package domain

import (
	"context"
	"errors"

	"github.com/bwmarrin/snowflake"
)

type Service interface {
	GetInvoiceForPublicView(ctx context.Context, orgID snowflake.ID, token string) (*PublicInvoiceResponse, error)
	GetInvoicePublicStatus(ctx context.Context, orgID snowflake.ID, token string) (PublicInvoiceStatus, error)
	CreateCheckoutSession(ctx context.Context, orgID snowflake.ID, token string, provider string) (*CheckoutSessionResponse, error)
	ProcessCheckoutSession(ctx context.Context, orgID snowflake.ID, token string, provider string, payload map[string]any) (*ProcessSessionResponse, error)
	ListPaymentMethods(ctx context.Context, orgID snowflake.ID) ([]PublicPaymentMethod, error)
}

type ProcessSessionResponse struct {
	Success        bool   `json:"success"`
	PaymentID      string `json:"payment_id,omitempty"`
	Status         string `json:"status,omitempty"`
	FailureMessage string `json:"failure_message,omitempty"`
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

type CheckoutSessionResponse struct {
	Provider     string         `json:"provider"`
	SessionToken string         `json:"session_token"`
	PublicConfig map[string]any `json:"public_config,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

var (
	ErrInvoiceUnavailable = errors.New("invoice_unavailable")
)

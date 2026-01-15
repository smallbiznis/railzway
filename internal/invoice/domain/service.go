package domain

import (
	"context"
	"errors"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/railzway/pkg/db/pagination"
)

type ListInvoiceRequest struct {
	Status        *InvoiceStatus
	InvoiceNumber *string
	CustomerID    *snowflake.ID
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
	DueFrom       *time.Time
	DueTo         *time.Time
	FinalizedFrom *time.Time
	FinalizedTo   *time.Time
	TotalMin      *int64
	TotalMax      *int64

	PageToken string
	PageSize  int
}

type ListInvoiceResponse struct {
	pagination.PageInfo
	Invoices []Invoice `json:"invoices"`
}

type RenderInvoiceResponse struct {
	InvoiceTemplateID *string `json:"invoice_template_id,omitempty"`
	RenderedHTML      string  `json:"rendered_html"`
	RenderedPDFURL    *string `json:"rendered_pdf_url,omitempty"`
	IsSnapshot        bool    `json:"is_snapshot"`
}

type Service interface {
	List(context.Context, ListInvoiceRequest) (ListInvoiceResponse, error)
	GetByID(ctx context.Context, id string) (Invoice, error)
	RenderInvoice(ctx context.Context, invoiceID string) (RenderInvoiceResponse, error)
	GenerateInvoice(ctx context.Context, billingCycleID string) (*Invoice, error)
	FinalizeInvoice(ctx context.Context, invoiceID string) error
	VoidInvoice(ctx context.Context, invoiceID string, reason string) error
}

var (
	ErrInvalidOrganization     = errors.New("invalid_organization")
	ErrInvalidBillingCycle     = errors.New("invalid_billing_cycle")
	ErrBillingCycleNotFound    = errors.New("billing_cycle_not_found")
	ErrBillingCycleNotClosed   = errors.New("billing_cycle_not_closed")
	ErrMissingLedgerEntry      = errors.New("missing_ledger_entry")
	ErrMissingRatingResults    = errors.New("missing_rating_results")
	ErrCurrencyMismatch        = errors.New("currency_mismatch")
	ErrInvalidInvoiceID        = errors.New("invalid_invoice_id")
	ErrInvalidSubtotal         = errors.New("invalid_subtotal_amount")
	ErrInvoiceNotFound         = errors.New("invoice_not_found")
	ErrInvoiceNotDraft         = errors.New("invoice_not_draft")
	ErrInvoiceNotFinalized     = errors.New("invoice_not_finalized")
	ErrInvoiceTemplateNotFound = errors.New("invoice_template_not_found")
	ErrInvoiceRenderMissing    = errors.New("invoice_render_missing")
)

package domain

import (
	"context"
	"errors"

	"github.com/smallbiznis/valora/pkg/db/pagination"
)

type ListInvoiceRequest struct{}

type ListInvoiceResponse struct {
	pagination.PageInfo
	Invoices []Invoice `json:"invoices"`
}

type Service interface {
	List(context.Context, ListInvoiceRequest) (ListInvoiceResponse, error)
	GetByID(ctx context.Context, id string) (Invoice, error)
	GenerateInvoice(ctx context.Context, billingCycleID string) error
	FinalizeInvoice(ctx context.Context, invoiceID string) error
	VoidInvoice(ctx context.Context, invoiceID string, reason string) error
}

var (
	ErrInvalidOrganization   = errors.New("invalid_organization")
	ErrInvalidBillingCycle   = errors.New("invalid_billing_cycle")
	ErrBillingCycleNotFound  = errors.New("billing_cycle_not_found")
	ErrBillingCycleNotClosed = errors.New("billing_cycle_not_closed")
	ErrMissingRatingResults  = errors.New("missing_rating_results")
	ErrCurrencyMismatch      = errors.New("currency_mismatch")
	ErrInvalidInvoiceID      = errors.New("invalid_invoice_id")
	ErrInvoiceNotFound       = errors.New("invoice_not_found")
	ErrInvoiceNotDraft       = errors.New("invoice_not_draft")
	ErrInvoiceNotFinalized   = errors.New("invoice_not_finalized")
)

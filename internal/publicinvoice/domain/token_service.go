package domain

import (
	"context"
	"errors"
	"time"

	"github.com/bwmarrin/snowflake"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
)

// PublicInvoiceTokenService ensures a finalized invoice has exactly one active public access token.
// Implementations must return the existing active token or create a new one when none exists.
// This service must not rotate tokens implicitly or mutate invoice state.
type PublicInvoiceTokenService interface {
	EnsureForInvoice(ctx context.Context, invoice invoicedomain.Invoice) (PublicInvoiceToken, error)
}

// PublicInvoiceTokenRepository abstracts persistence for public invoice tokens.
// Implementations must guarantee at most one active token per invoice.
type PublicInvoiceTokenRepository interface {
	FindActiveByInvoiceID(ctx context.Context, invoiceID snowflake.ID) (*PublicInvoiceToken, error)
	Create(ctx context.Context, token PublicInvoiceToken) error
}

// PublicInvoiceToken represents a public access token for an invoice.
// The raw token is returned only once by the service.
// OrgID is persisted for lookup but is not a security boundary.
type PublicInvoiceToken struct {
	ID        snowflake.ID
	OrgID     snowflake.ID
	InvoiceID snowflake.ID
	TokenHash     string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

var (
	// ErrInvoiceNotFinalized indicates the invoice is not eligible for public tokens.
	ErrInvoiceNotFinalized = errors.New("invoice_not_finalized")
	// ErrInvoiceVoided indicates the invoice has been voided and is not eligible.
	ErrInvoiceVoided = errors.New("invoice_voided")
	// ErrPublicTokenAlreadyExists indicates an invariant violation when a new token would be created.
	ErrPublicTokenAlreadyExists = errors.New("public_token_already_exists")
	// ErrInvariantViolation indicates a domain invariant was violated.
	ErrInvariantViolation = errors.New("invariant_violation")
)

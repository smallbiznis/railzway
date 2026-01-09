package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	publicinvoicedomain "github.com/smallbiznis/valora/internal/publicinvoice/domain"
	"go.uber.org/fx"
)

type TokenParams struct {
	fx.In

	Repo  publicinvoicedomain.PublicInvoiceTokenRepository
	GenID *snowflake.Node
}

type TokenService struct {
	repo  publicinvoicedomain.PublicInvoiceTokenRepository
	genID *snowflake.Node
}

func NewTokenService(p TokenParams) publicinvoicedomain.PublicInvoiceTokenService {
	return &TokenService{
		repo:  p.Repo,
		genID: p.GenID,
	}
}

func (s *TokenService) EnsureForInvoice(
	ctx context.Context,
	invoice invoicedomain.Invoice,
) (publicinvoicedomain.PublicInvoiceToken, error) {
	if invoice.ID == 0 || invoice.OrgID == 0 {
		return publicinvoicedomain.PublicInvoiceToken{}, publicinvoicedomain.ErrInvariantViolation
	}

	status := strings.ToUpper(strings.TrimSpace(string(invoice.Status)))
	switch status {
	case string(invoicedomain.InvoiceStatusVoid):
		return publicinvoicedomain.PublicInvoiceToken{}, publicinvoicedomain.ErrInvoiceVoided
	case string(invoicedomain.InvoiceStatusDraft):
		return publicinvoicedomain.PublicInvoiceToken{}, publicinvoicedomain.ErrInvoiceNotFinalized
	case string(invoicedomain.InvoiceStatusFinalized), "ISSUED", "PAID":
		// allowed
	default:
		return publicinvoicedomain.PublicInvoiceToken{}, publicinvoicedomain.ErrInvoiceNotFinalized
	}

	existing, err := s.repo.FindActiveByInvoiceID(ctx, invoice.ID)
	if err != nil {
		return publicinvoicedomain.PublicInvoiceToken{}, err
	}
	if existing != nil {
		if strings.TrimSpace(existing.TokenHash) == "" {
			return publicinvoicedomain.PublicInvoiceToken{}, publicinvoicedomain.ErrInvariantViolation
		}
		return *existing, nil
	}

	rawToken, err := generateToken()
	if err != nil {
		return publicinvoicedomain.PublicInvoiceToken{}, err
	}

	now := time.Now().UTC()
	newToken := publicinvoicedomain.PublicInvoiceToken{
		ID:        s.genID.Generate(),
		OrgID:     invoice.OrgID,
		InvoiceID: invoice.ID,
		TokenHash: rawToken,
		CreatedAt: now,
	}

	fmt.Printf("raw_token: %v\n", rawToken)
	if err := s.repo.Create(ctx, newToken); err != nil {
		fallback, fetchErr := s.repo.FindActiveByInvoiceID(ctx, invoice.ID)
		if fetchErr == nil && fallback != nil && strings.TrimSpace(fallback.TokenHash) != "" {
			return *fallback, nil
		}
		return publicinvoicedomain.PublicInvoiceToken{}, err
	}

	return newToken, nil
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

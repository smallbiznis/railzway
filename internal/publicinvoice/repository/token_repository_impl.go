package repository

import (
	"context"
	"strings"

	"github.com/bwmarrin/snowflake"
	publicinvoicedomain "github.com/smallbiznis/valora/internal/publicinvoice/domain"
	"gorm.io/gorm"
)

type tokenRepo struct {
	db *gorm.DB
}

func ProvideTokenRepository(db *gorm.DB) publicinvoicedomain.PublicInvoiceTokenRepository {
	return &tokenRepo{db: db}
}

func (r *tokenRepo) FindActiveByInvoiceID(
	ctx context.Context,
	invoiceID snowflake.ID,
) (*publicinvoicedomain.PublicInvoiceToken, error) {
	if invoiceID == 0 {
		return nil, nil
	}

	var row publicinvoicedomain.PublicInvoiceToken
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id, org_id, invoice_id, token_hash, created_at, expires_at
		 FROM invoice_public_tokens
		 WHERE invoice_id = ? AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY created_at DESC
		 LIMIT 1`,
		invoiceID,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	row.TokenHash = strings.TrimSpace(row.TokenHash)
	return &row, nil
}

func (r *tokenRepo) Create(ctx context.Context, token publicinvoicedomain.PublicInvoiceToken) error {
	if token.ID == 0 || token.OrgID == 0 || token.InvoiceID == 0 {
		return publicinvoicedomain.ErrInvariantViolation
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO invoice_public_tokens (
			id, org_id, invoice_id, token_hash, expires_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?)`,
		token.ID,
		token.OrgID,
		token.InvoiceID,
		hashToken(token.TokenHash),
		token.ExpiresAt,
		token.CreatedAt,
	).Error
}

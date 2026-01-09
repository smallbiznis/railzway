package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	publicinvoicedomain "github.com/smallbiznis/valora/internal/publicinvoice/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type repo struct{}

func Provide() publicinvoicedomain.Repository {
	return &repo{}
}

func (r *repo) FindInvoiceByToken(
	ctx context.Context,
	db *gorm.DB,
	orgID snowflake.ID,
	token string,
) (*publicinvoicedomain.InvoiceRecord, error) {
	if db == nil || orgID == 0 || token == "" {
		return nil, nil
	}
	tokenHash := hashToken(token)

	query := `
		SELECT i.id, i.org_id, i.invoice_number, i.status, i.subtotal_amount, i.currency,
			i.issued_at, i.due_at, i.paid_at, i.customer_id, i.metadata,
			o.name AS org_name, c.name AS customer_name, c.email AS customer_email
		FROM invoice_public_tokens t
		JOIN invoices i ON i.id = t.invoice_id
		JOIN organizations o ON o.id = i.org_id
		LEFT JOIN customers c ON c.id = i.customer_id
		WHERE t.token_hash = ? AND t.revoked_at IS NULL
			AND (t.expires_at IS NULL OR t.expires_at > NOW())
		LIMIT 1`

	var row publicinvoicedomain.InvoiceRecord
	if err := db.WithContext(ctx).Raw(query, tokenHash).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *repo) ListInvoiceItems(
	ctx context.Context,
	db *gorm.DB,
	orgID snowflake.ID,
	invoiceID snowflake.ID,
) ([]publicinvoicedomain.InvoiceItemRecord, error) {
	if db == nil || orgID == 0 || invoiceID == 0 {
		return nil, nil
	}

	var rows []publicinvoicedomain.InvoiceItemRecord
	if err := db.WithContext(ctx).Raw(
		`SELECT description, quantity, unit_price, amount, line_type
		 FROM invoice_items
		 WHERE invoice_id = ? AND org_id = ?
		 ORDER BY created_at ASC, id ASC`,
		invoiceID,
		orgID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *repo) ListPaymentMethods(
	ctx context.Context,
	db *gorm.DB,
	orgID snowflake.ID,
) ([]publicinvoicedomain.PaymentMethodRecord, error) {
	if db == nil || orgID == 0 {
		return nil, nil
	}

	var rows []publicinvoicedomain.PaymentMethodRecord
	if err := db.WithContext(ctx).Raw(
		`SELECT c.provider, catalog.display_name
		 FROM payment_provider_configs c
		 JOIN payment_provider_catalog catalog ON catalog.provider = c.provider
		 WHERE c.org_id = ? AND c.is_active = TRUE
		 ORDER BY catalog.display_name ASC`,
		orgID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *repo) UpdateInvoiceMetadata(
	ctx context.Context,
	db *gorm.DB,
	orgID snowflake.ID,
	invoiceID snowflake.ID,
	metadata datatypes.JSONMap,
	updatedAt time.Time,
) error {
	if db == nil || orgID == 0 || invoiceID == 0 {
		return nil
	}

	return db.WithContext(ctx).Exec(
		`UPDATE invoices
		 SET metadata = ?, updated_at = ?
		 WHERE id = ? AND org_id = ?`,
		metadata,
		updatedAt,
		invoiceID,
		orgID,
	).Error
}

func (r *repo) FindInvoiceSettledAmount(
	ctx context.Context,
	db *gorm.DB,
	orgID snowflake.ID,
	invoiceID snowflake.ID,
	currency string,
) (int64, error) {
	if db == nil || orgID == 0 || invoiceID == 0 {
		return 0, nil
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return 0, nil
	}

	invoiceIDText := invoiceID.String()
	var settled int64

	err := db.WithContext(ctx).Raw(
		`
		SELECT COALESCE(
			SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END),
			0
		) AS settled_amount
		FROM ledger_entries le
		JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
		JOIN ledger_accounts a ON a.id = l.account_id
		JOIN payment_events pe ON pe.id = le.source_id
		WHERE le.org_id = ?
		  AND le.currency = ?
		  AND le.source_type = ?
		  AND a.code = ?
		  AND (pe.payload #>> '{data,object,metadata,invoice_id}') = ?
		`,
		orgID,
		currency,
		ledgerdomain.SourceTypePayment,
		ledgerdomain.AccountCodeAccountsReceivable,
		invoiceIDText,
	).Scan(&settled).Error
	if err != nil {
		return 0, err
	}

	return settled, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

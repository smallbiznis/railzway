package repository

import (
	"context"
	"time"

	"strings"

	"github.com/bwmarrin/snowflake"
	billingopsdomain "github.com/smallbiznis/railzway/internal/billingoperations/domain"
	ledgerdomain "github.com/smallbiznis/railzway/internal/ledger/domain"
	paymentdomain "github.com/smallbiznis/railzway/internal/payment/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type RepositoryImpl struct {
	db *gorm.DB
	// FinOps repo can be embedded or composed
	finOpsRepo *FinOpsSnapshotRepository
}

func NewRepository(db *gorm.DB) billingopsdomain.Repository {
	return &RepositoryImpl{
		db:         db,
		finOpsRepo: NewFinOpsSnapshotRepository(db),
	}
}

func (r *RepositoryImpl) WithTx(tx *gorm.DB) billingopsdomain.Repository {
	return &RepositoryImpl{
		db:         tx,
		finOpsRepo: NewFinOpsSnapshotRepository(tx),
	}
}

func (r *RepositoryImpl) FetchOrgCurrency(ctx context.Context, orgID snowflake.ID) (string, error) {
	var row struct {
		Currency string `gorm:"column:currency"`
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT currency FROM organization_billing_preferences WHERE org_id = ? LIMIT 1`,
		orgID,
	).Scan(&row).Error; err != nil {
		return "", err
	}
	currency := strings.ToUpper(strings.TrimSpace(row.Currency))
	if currency == "" {
		currency = "USD"
	}
	return currency, nil
}

func (r *RepositoryImpl) ListOverdueInvoices(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
	limit int,
) ([]billingopsdomain.OverdueInvoiceRow, error) {
	var rows []billingopsdomain.OverdueInvoiceRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		)
		SELECT
			i.id AS invoice_id,
			COALESCE(i.invoice_number::text, '') AS invoice_number,
			c.id AS customer_id,
			c.name AS customer_name,
			GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS amount_due,
			i.due_at AS due_at,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at,
			ipt.token_hash AS token_hash
		FROM invoices i
		JOIN customers c ON c.id = i.customer_id
		LEFT JOIN settled s ON s.invoice_id_text = i.id::text
		LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = i.id AND ipt.revoked_at IS NULL
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?
			AND boa.entity_id = i.id
			AND boa.status != 'released'
		WHERE i.org_id = ?
		  AND i.status = 'FINALIZED'
		  AND i.voided_at IS NULL
		  AND i.paid_at IS NULL
		  AND i.currency = ?
		  AND i.due_at IS NOT NULL
		  AND i.due_at < ?
		  AND GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) > 0
		ORDER BY i.due_at ASC
		LIMIT ?`

	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		billingopsdomain.EntityTypeInvoice,
		orgID,
		currency,
		now,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) ListOutstandingCustomers(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
	limit int,
) ([]billingopsdomain.OutstandingCustomerRow, error) {
	var rows []billingopsdomain.OutstandingCustomerRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), invoice_outstanding AS (
			SELECT
				i.id AS invoice_id,
				i.customer_id,
				COALESCE(i.invoice_number::text, '') AS invoice_number,
				i.due_at,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
			FROM invoices i
			LEFT JOIN settled s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
			  AND i.status = 'FINALIZED'
			  AND i.voided_at IS NULL
			  AND i.currency = ?
		), totals AS (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM invoice_outstanding
			WHERE outstanding > 0
			GROUP BY customer_id
		), oldest_overdue AS (
			SELECT DISTINCT ON (customer_id)
				customer_id,
				invoice_id,
				invoice_number,
				due_at
			FROM invoice_outstanding
			WHERE outstanding > 0 AND due_at IS NOT NULL AND due_at < ?
			ORDER BY customer_id, due_at ASC, invoice_id ASC
		), last_payment AS (
			SELECT customer_id, MAX(received_at) AS last_payment_at
			FROM payment_events
			WHERE org_id = ? AND event_type = 'payment_succeeded'
			GROUP BY customer_id
		)
		SELECT
			c.id AS customer_id,
			c.name AS customer_name,
			t.outstanding AS outstanding,
			oo.invoice_id::text AS oldest_overdue_invoice_id,
			oo.invoice_number AS oldest_overdue_invoice_number,
			oo.due_at AS oldest_overdue_at,
			lp.last_payment_at AS last_payment_at,
			ipt.token_hash AS token_hash,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at
		FROM totals t
		JOIN customers c ON c.id = t.customer_id
		LEFT JOIN oldest_overdue oo ON oo.customer_id = t.customer_id
		LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = oo.invoice_id AND ipt.revoked_at IS NULL
		LEFT JOIN last_payment lp ON lp.customer_id = t.customer_id
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?
			AND boa.entity_id = c.id
			AND boa.status != 'released'
		WHERE c.org_id = ?
		ORDER BY t.outstanding DESC
		LIMIT ?`

	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		currency,
		now,
		orgID,
		orgID,
		billingopsdomain.EntityTypeCustomer,
		orgID,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) ListPaymentIssues(ctx context.Context, orgID snowflake.ID, now time.Time, limit int) ([]billingopsdomain.PaymentIssueRow, error) {
	var rows []billingopsdomain.PaymentIssueRow
	query := `
		SELECT
			pe.customer_id AS customer_id,
			c.name AS customer_name,
			pe.event_type AS issue_type,
			MAX(pe.received_at) AS last_attempt,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at
		FROM payment_events pe
		JOIN customers c ON c.id = pe.customer_id
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?
			AND boa.entity_id = pe.customer_id
			AND boa.status != 'released'
		WHERE pe.org_id = ?
		  AND pe.event_type = ?
		GROUP BY pe.customer_id, c.name, pe.event_type, boa.assigned_to, boa.assigned_at, boa.assignment_expires_at, boa.status, boa.released_at, boa.released_by, boa.release_reason, boa.last_action_at
		ORDER BY last_attempt DESC
		LIMIT ?`

	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID,
		billingopsdomain.EntityTypeCustomer,
		orgID,
		paymentdomain.EventTypePaymentFailed,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) LoadActionSummary(ctx context.Context, orgID snowflake.ID, currency string, now time.Time) (billingopsdomain.ActionSummaryRow, error) {
	var row billingopsdomain.ActionSummaryRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), invoice_outstanding AS (
			SELECT
				i.id AS invoice_id,
				i.customer_id,
				i.due_at,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
			FROM invoices i
			LEFT JOIN settled s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
			  AND i.status = 'FINALIZED'
			  AND i.voided_at IS NULL
			  AND i.currency = ?
		), totals AS (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM invoice_outstanding
			WHERE outstanding > 0
			GROUP BY customer_id
		)
		SELECT
			COALESCE((SELECT COUNT(*) FROM totals), 0) AS customers_with_outstanding,
			COALESCE((SELECT COUNT(*) FROM invoice_outstanding WHERE outstanding > 0 AND due_at IS NOT NULL AND due_at < ?), 0) AS overdue_invoices,
			COALESCE((SELECT COUNT(*) FROM payment_events WHERE org_id = ? AND event_type = ?), 0) AS failed_payment_attempts,
			COALESCE((SELECT SUM(outstanding) FROM totals), 0) AS total_outstanding`

	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		currency,
		now,
		orgID,
		paymentdomain.EventTypePaymentFailed,
	).Scan(&row).Error; err != nil {
		return billingopsdomain.ActionSummaryRow{}, err
	}
	return row, nil
}

func (r *RepositoryImpl) ListCollectionQueue(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
	limit int,
) ([]billingopsdomain.CollectionQueueRow, error) {
	var rows []billingopsdomain.CollectionQueueRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), invoice_outstanding AS (
			SELECT
				i.id AS invoice_id,
				i.customer_id,
				COALESCE(i.invoice_number::text, '') AS invoice_number,
				i.due_at,
				COALESCE(i.issued_at, i.created_at) AS issued_at,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
			FROM invoices i
			LEFT JOIN settled s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
			  AND i.status = 'FINALIZED'
			  AND i.voided_at IS NULL
			  AND i.currency = ?
		), totals AS (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM invoice_outstanding
			WHERE outstanding > 0
			GROUP BY customer_id
		), oldest_unpaid AS (
			SELECT DISTINCT ON (customer_id)
				customer_id,
				invoice_id,
				invoice_number,
				due_at,
				issued_at
			FROM invoice_outstanding
			WHERE outstanding > 0
			ORDER BY customer_id, COALESCE(due_at, issued_at) ASC, invoice_id ASC
		), last_payment AS (
			SELECT customer_id, MAX(received_at) AS last_payment_at
			FROM payment_events
			WHERE org_id = ? AND event_type = 'payment_succeeded'
			GROUP BY customer_id
		)
		SELECT
			c.id AS customer_id,
			c.name AS customer_name,
			t.outstanding AS outstanding,
			ou.invoice_id::text AS oldest_unpaid_invoice_id,
			ou.invoice_number AS oldest_unpaid_invoice_number,
			ou.due_at AS oldest_unpaid_at,
			lp.last_payment_at AS last_payment_at,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at,
			ipt.token_hash AS token_hash
		FROM totals t
		JOIN customers c ON c.id = t.customer_id
		LEFT JOIN oldest_unpaid ou ON ou.customer_id = t.customer_id
		LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = ou.invoice_id AND ipt.revoked_at IS NULL
		LEFT JOIN last_payment lp ON lp.customer_id = t.customer_id
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?
			AND boa.entity_id = c.id
			AND boa.status != 'released'
		WHERE c.org_id = ?
		ORDER BY
			CASE
				WHEN ou.due_at IS NULL THEN 1
				WHEN ou.due_at <= (?::timestamptz - interval '60 days') THEN 3
				WHEN ou.due_at <= (?::timestamptz - interval '31 days') THEN 2
				ELSE 1
			END DESC,
			t.outstanding DESC,
			c.id ASC
		LIMIT ?`

	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		currency,
		orgID,
		orgID,
		billingopsdomain.EntityTypeCustomer,
		orgID,
		now,
		now,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) ListFailedPaymentActions(
	ctx context.Context,
	orgID snowflake.ID,
	currency string,
	now time.Time,
	limit int,
) ([]billingopsdomain.FailedPaymentActionRow, error) {
	var rows []billingopsdomain.FailedPaymentActionRow
	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), failed AS (
			SELECT
				pe.customer_id AS customer_id,
				c.name AS customer_name,
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				MAX(pe.received_at) AS last_attempt
			FROM payment_events pe
			JOIN customers c ON c.id = pe.customer_id
			WHERE pe.org_id = ?
			  AND pe.event_type = ?
			GROUP BY pe.customer_id, c.name, invoice_id_text
		)
		SELECT
			f.customer_id AS customer_id,
			f.customer_name AS customer_name,
			f.invoice_id_text AS invoice_id,
			COALESCE(i.invoice_number::text, '') AS invoice_number,
			GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS amount_due,
			i.due_at AS due_at,
			f.last_attempt AS last_attempt,
			boa.assigned_to AS assigned_to,
			boa.assigned_at AS assigned_at,
			boa.assignment_expires_at AS assignment_expires_at,
			boa.status AS assignment_status,
			boa.released_at AS assignment_released_at,
			boa.released_by AS assignment_released_by,
			boa.release_reason AS assignment_release_reason,
			boa.last_action_at AS assignment_last_action_at,
			ipt.token_hash AS token_hash
		FROM failed f
		LEFT JOIN invoices i
			ON i.id::text = f.invoice_id_text
			AND i.org_id = ?
			AND i.status = 'FINALIZED'
			AND i.voided_at IS NULL
			AND i.currency = ?
		LEFT JOIN settled s ON s.invoice_id_text = i.id::text
		LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = i.id AND ipt.revoked_at IS NULL
		LEFT JOIN billing_operation_assignments boa
			ON boa.org_id = ?
			AND boa.entity_type = ?

			AND boa.entity_id = f.customer_id
			AND boa.status != 'released'
		WHERE (i.id IS NULL OR GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) > 0)
		ORDER BY f.last_attempt DESC
		LIMIT ?`

	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		paymentdomain.EventTypePaymentFailed,
		orgID,
		currency,
		orgID,
		billingopsdomain.EntityTypeCustomer,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) GenerateActionID() snowflake.ID {
	// Snowflake generation usually requires a node.
	// For repository to generate ID, the node needs to be passed or injected.
	// Current service implementation uses s.genID.
	// We might prefer to keep ID generation in service layer.
	// However, if we need ID for pure DB logic, we pass it.
	return 0
}

func (r *RepositoryImpl) InsertBillingAction(ctx context.Context, record billingopsdomain.BillingActionRecord) (bool, error) {
	if record.ID == 0 {
		return false, billingopsdomain.ErrInvalidEntityID
	}

	var idempotencyValue any
	if record.IdempotencyKey != "" {
		idempotencyValue = record.IdempotencyKey
	}
	metadata := record.Metadata
	if metadata == nil {
		metadata = datatypes.JSONMap{}
	}
	result := r.db.WithContext(ctx).Exec(
		`INSERT INTO billing_operation_actions (
			id, org_id, entity_type, entity_id, action_type, action_bucket,
			idempotency_key, metadata, actor_type, actor_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT DO NOTHING`,
		record.ID,
		record.OrgID,
		record.EntityType,
		record.EntityID,
		record.ActionType,
		record.ActionBucket,
		idempotencyValue,
		metadata,
		strings.TrimSpace(record.ActorType),
		strings.TrimSpace(record.ActorID),
		record.CreatedAt,
	)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *RepositoryImpl) FindActionByIdempotencyKey(ctx context.Context, orgID snowflake.ID, key string) (*billingopsdomain.BillingActionLookup, error) {
	var row billingopsdomain.BillingActionLookup
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM billing_operation_actions WHERE org_id = ? AND idempotency_key = ? LIMIT 1`,
		orgID,
		key,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *RepositoryImpl) FindActionByBucket(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID, actionType string, bucket time.Time) (*billingopsdomain.BillingActionLookup, error) {
	var row billingopsdomain.BillingActionLookup
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM billing_operation_actions
		 WHERE org_id = ? AND entity_type = ? AND entity_id = ? AND action_type = ? AND action_bucket = ?
		 LIMIT 1`,
		orgID,
		entityType,
		entityID,
		actionType,
		bucket,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *RepositoryImpl) LoadAssignment(ctx context.Context, orgID snowflake.ID, entityType string, entityID snowflake.ID) (*billingopsdomain.AssignmentRow, error) {
	var row billingopsdomain.AssignmentRow
	if err := r.db.WithContext(ctx).Raw(
		`SELECT assigned_to, assigned_at, assignment_expires_at,
		        status, released_at, released_by, release_reason, last_action_at
		 FROM billing_operation_assignments
		 WHERE org_id = ? AND entity_type = ? AND entity_id = ?
		 LIMIT 1`,
		orgID,
		entityType,
		entityID,
	).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.AssignedTo == "" {
		return nil, nil
	}
	return &row, nil
}

func (r *RepositoryImpl) LoadAssignmentForUpdate(
	ctx context.Context,
	orgID snowflake.ID,
	entityType string,
	entityID snowflake.ID,
) (*billingopsdomain.BillingAssignmentRecord, error) {
	var row billingopsdomain.BillingAssignmentRecord
	query := `SELECT id, org_id, entity_type, entity_id,
		        assigned_to, assigned_at, assignment_expires_at,
		        status, released_at, released_by, release_reason, last_action_at,
				snapshot_metadata, created_at, updated_at
		 FROM billing_operation_assignments
		 WHERE org_id = ? AND entity_type = ? AND entity_id = ?`

	if r.db.Dialector.Name() != "sqlite" {
		query += " FOR UPDATE"
	}

	err := r.db.WithContext(ctx).Raw(query, orgID, entityType, entityID).Scan(&row).Error

	if err != nil {
		return nil, err
	}
	if row.AssignedTo == "" {
		return nil, nil
	}
	return &row, nil
}

func (r *RepositoryImpl) ListActiveAssignments(ctx context.Context) ([]billingopsdomain.BillingAssignmentRecord, error) {
	var records []billingopsdomain.BillingAssignmentRecord
	if err := r.db.WithContext(ctx).Where("status IN ? AND breached_at IS NULL",
		[]string{billingopsdomain.AssignmentStatusAssigned, billingopsdomain.AssignmentStatusInProgress}).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (r *RepositoryImpl) UpsertAssignment(
	ctx context.Context,
	record billingopsdomain.BillingAssignmentRecord,
) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO billing_operation_assignments (
			id, org_id, entity_type, entity_id,
			assigned_to, assigned_at, assignment_expires_at,
			status, released_at, released_by, release_reason, last_action_at,
			snapshot_metadata,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (org_id, entity_type, entity_id) DO UPDATE SET
			assigned_to = EXCLUDED.assigned_to,
			assigned_at = EXCLUDED.assigned_at,
			assignment_expires_at = EXCLUDED.assignment_expires_at,
			status = EXCLUDED.status,
			released_at = EXCLUDED.released_at,
			released_by = EXCLUDED.released_by,
			release_reason = EXCLUDED.release_reason,
			last_action_at = EXCLUDED.last_action_at,
			snapshot_metadata = EXCLUDED.snapshot_metadata,
			updated_at = EXCLUDED.updated_at`,
		record.ID,
		record.OrgID,
		record.EntityType,
		record.EntityID,
		record.AssignedTo,
		record.AssignedAt,
		record.AssignmentExpiresAt,
		record.Status,
		record.ReleasedAt,
		record.ReleasedBy,
		record.ReleaseReason,
		record.LastActionAt,
		record.SnapshotMetadata,
		record.CreatedAt,
		record.UpdatedAt,
	).Error
}

func (r *RepositoryImpl) UpdateAssignmentStatus(
	ctx context.Context,
	orgID snowflake.ID,
	entityType string,
	entityID snowflake.ID,
	oldStatus, newStatus string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE billing_operation_assignments
		 SET status = ?, last_action_at = ?, updated_at = ?
		 WHERE org_id = ? AND entity_type = ? AND entity_id = ? 
		   AND status = ?`,
		newStatus, now, now,
		orgID, entityType, entityID,
		oldStatus,
	).Error
}

func (r *RepositoryImpl) EscalateAssignment(
	ctx context.Context,
	orgID snowflake.ID,
	entityType string,
	entityID snowflake.ID,
	breachType string,
	now time.Time,
) error {
	return r.db.WithContext(ctx).Model(&billingopsdomain.BillingAssignmentRecord{}).
		Where("org_id = ? AND entity_type = ? AND entity_id = ?", orgID, entityType, entityID).
		Updates(map[string]interface{}{
			"status":       billingopsdomain.AssignmentStatusEscalated,
			"breached_at":  now,
			"breach_level": breachType,
			"resolved_at":  now,
			"resolved_by":  "system",
			"updated_at":   now,
		}).Error
}

func (r *RepositoryImpl) FindSnapshotsByUser(ctx context.Context, orgID snowflake.ID, userID string, periodType string, start, end time.Time) ([]billingopsdomain.FinOpsScoreSnapshot, error) {
	return r.finOpsRepo.FindByUser(ctx, orgID, userID, periodType, start, end)
}

func (r *RepositoryImpl) FindSnapshotsByUserWithLimit(ctx context.Context, orgID snowflake.ID, userID string, periodType string, start, end time.Time, limit int) ([]billingopsdomain.FinOpsScoreSnapshot, error) {
	return r.finOpsRepo.FindByUserWithLimit(ctx, orgID, userID, periodType, start, end, limit)
}

func (r *RepositoryImpl) FindSnapshotsByOrg(ctx context.Context, orgID snowflake.ID, periodType string, start, end time.Time) ([]billingopsdomain.FinOpsScoreSnapshot, error) {
	return r.finOpsRepo.FindByOrg(ctx, orgID, periodType, start, end)
}

func (r *RepositoryImpl) LoadEntitySnapshot(
	ctx context.Context,
	orgID snowflake.ID,
	entityType string,
	entityID snowflake.ID,
) (map[string]any, error) {
	now := time.Now().UTC()
	switch entityType {
	case billingopsdomain.EntityTypeInvoice:
		return r.loadInvoiceSnapshot(ctx, orgID, entityID, now)
	case billingopsdomain.EntityTypeCustomer:
		return r.loadCustomerSnapshot(ctx, orgID, entityID, now)
	default:
		return nil, nil
	}
}

func (r *RepositoryImpl) loadInvoiceSnapshot(ctx context.Context, orgID, invoiceID snowflake.ID, now time.Time) (map[string]any, error) {
	var row struct {
		InvoiceID     snowflake.ID `gorm:"column:invoice_id"`
		InvoiceNumber string       `gorm:"column:invoice_number"`
		Status        string       `gorm:"column:status"`
		CustomerID    snowflake.ID `gorm:"column:customer_id"`
		CustomerName  string       `gorm:"column:customer_name"`
		Currency      string       `gorm:"column:currency"`
		AmountDue     int64        `gorm:"column:amount_due"`
		DueAt         *time.Time   `gorm:"column:due_at"`
	}
	query := `
		SELECT
			i.id AS invoice_id,
			COALESCE(i.invoice_number::text, '') AS invoice_number,
			i.status AS status,
			i.customer_id AS customer_id,
			c.name AS customer_name,
			i.currency AS currency,
			i.due_at AS due_at,
			GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS amount_due
		FROM invoices i
		JOIN customers c ON c.id = i.customer_id
		LEFT JOIN (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = (SELECT currency FROM invoices WHERE id = ? AND org_id = ?)
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		) s ON s.invoice_id_text = i.id::text
		WHERE i.org_id = ? AND i.id = ?
		LIMIT 1`

	currency, err := r.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID,
		invoiceID, orgID,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		invoiceID,
	).Scan(&row).Error; err != nil {
		return nil, err
	}

	if row.InvoiceID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	snapshot := map[string]any{
		"invoice_id":     row.InvoiceID.String(),
		"invoice_number": strings.TrimSpace(row.InvoiceNumber),
		"status":         row.Status,
		"customer_id":    row.CustomerID.String(),
		"customer_name":  row.CustomerName,
		"amount_due":     row.AmountDue,
		"currency":       strings.ToUpper(strings.TrimSpace(row.Currency)),
	}
	if row.DueAt != nil {
		due := row.DueAt.UTC()
		daysOverdue := int(now.Sub(due).Hours() / 24)
		if daysOverdue < 0 {
			daysOverdue = 0
		}
		snapshot["due_at"] = due.Format(time.RFC3339)
		snapshot["days_overdue"] = daysOverdue
	}
	return snapshot, nil
}

func (r *RepositoryImpl) loadCustomerSnapshot(ctx context.Context, orgID, customerID snowflake.ID, now time.Time) (map[string]any, error) {
	currency, err := r.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return nil, err
	}

	var row struct {
		CustomerID            snowflake.ID `gorm:"column:customer_id"`
		CustomerName          string       `gorm:"column:customer_name"`
		Outstanding           int64        `gorm:"column:outstanding"`
		OldestUnpaidInvoiceID *string      `gorm:"column:oldest_unpaid_invoice_id"`
		OldestUnpaidInvoice   *string      `gorm:"column:oldest_unpaid_invoice_number"`
		OldestUnpaidAt        *time.Time   `gorm:"column:oldest_unpaid_at"`
		LastPaymentAt         *time.Time   `gorm:"column:last_payment_at"`
	}

	query := `
		WITH settled AS (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ?
			  AND le.currency = ?
			  AND le.source_type = ?
			  AND a.code = ?
			GROUP BY 1
		), invoice_outstanding AS (
			SELECT
				i.id AS invoice_id,
				i.customer_id,
				COALESCE(i.invoice_number::text, '') AS invoice_number,
				i.due_at,
				COALESCE(i.issued_at, i.created_at) AS issued_at,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
			FROM invoices i
			LEFT JOIN settled s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
			  AND i.status = 'FINALIZED'
			  AND i.voided_at IS NULL
			  AND i.currency = ?
		), totals AS (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM invoice_outstanding
			WHERE outstanding > 0
			GROUP BY customer_id
		), oldest_unpaid AS (
			SELECT DISTINCT ON (customer_id)
				customer_id,
				invoice_id,
				invoice_number,
				due_at,
				issued_at
			FROM invoice_outstanding
			WHERE outstanding > 0
			ORDER BY customer_id, COALESCE(due_at, issued_at) ASC, invoice_id ASC
		), last_payment AS (
			SELECT customer_id, MAX(received_at) AS last_payment_at
			FROM payment_events
			WHERE org_id = ? AND event_type = 'payment_succeeded'
			GROUP BY customer_id
		)
		SELECT
			c.id AS customer_id,
			c.name AS customer_name,
			COALESCE(t.outstanding, 0) AS outstanding,
			ou.invoice_id::text AS oldest_unpaid_invoice_id,
			ou.invoice_number AS oldest_unpaid_invoice_number,
			ou.due_at AS oldest_unpaid_at,
			lp.last_payment_at AS last_payment_at
		FROM customers c
		LEFT JOIN totals t ON t.customer_id = c.id
		LEFT JOIN oldest_unpaid ou ON ou.customer_id = c.id
		LEFT JOIN last_payment lp ON lp.customer_id = c.id
		WHERE c.org_id = ? AND c.id = ?
		LIMIT 1`

	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID,
		currency,
		string(ledgerdomain.SourceTypePayment),
		string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID,
		currency,
		orgID,
		orgID,
		customerID,
	).Scan(&row).Error; err != nil {
		return nil, err
	}

	if row.CustomerID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	oldestDays := 0
	if row.OldestUnpaidAt != nil {
		due := row.OldestUnpaidAt.UTC()
		oldestDays = int(now.Sub(due).Hours() / 24)
		if oldestDays < 0 {
			oldestDays = 0
		}
	}

	snapshot := map[string]any{
		"customer_id":         row.CustomerID.String(),
		"customer_name":       row.CustomerName,
		"outstanding_balance": row.Outstanding,
		"currency":            currency,
		"oldest_unpaid_days":  oldestDays,
		"aging_bucket":        computeAgingBucket(oldestDays),
		"risk_level":          computeRiskLevel(row.Outstanding, oldestDays),
	}
	if row.OldestUnpaidAt != nil {
		snapshot["oldest_unpaid_at"] = row.OldestUnpaidAt.UTC().Format(time.RFC3339)
	}
	if row.OldestUnpaidInvoiceID != nil {
		snapshot["oldest_unpaid_invoice_id"] = strings.TrimSpace(*row.OldestUnpaidInvoiceID)
	}
	if row.OldestUnpaidInvoice != nil {
		snapshot["oldest_unpaid_invoice_number"] = strings.TrimSpace(*row.OldestUnpaidInvoice)
	}
	if row.LastPaymentAt != nil {
		snapshot["last_payment_at"] = row.LastPaymentAt.UTC().Format(time.RFC3339)
	}
	return snapshot, nil
}

func computeAgingBucket(days int) string {
	switch {
	case days >= 60:
		return "60+"
	case days >= 31:
		return "31-60"
	default:
		return "0-30"
	}
}

func computeRiskLevel(outstanding int64, oldestDays int) string {
	switch {
	case oldestDays >= 60 || outstanding >= 1000000:
		return "high"
	case oldestDays >= 31 || outstanding >= 250000:
		return "medium"
	default:
		return "low"
	}
}

func (r *RepositoryImpl) ListInboxItems(
	ctx context.Context,
	orgID snowflake.ID,
	limit int,
	now time.Time,
) ([]billingopsdomain.InboxRow, error) {
	query := `
		WITH risky_invoices AS (
			SELECT
				'invoice' AS entity_type,
				i.id::text AS entity_id,
				COALESCE(i.invoice_number::text, i.id::text) AS entity_name,
				'overdue' AS risk_category,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS amount_due,
				i.due_at,
				EXTRACT(EPOCH FROM (? - i.due_at)) / 86400 AS days_overdue,
				NULL::timestamp AS last_attempt,
				ipt.token_hash,
				-- Risk score: higher = more urgent
				(EXTRACT(EPOCH FROM (? - i.due_at)) / 86400 * 10 + i.subtotal_amount / 10000)::int AS risk_score
			FROM invoices i
			LEFT JOIN (
				SELECT
					(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
					SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
				FROM ledger_entries le
				JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
				JOIN ledger_accounts a ON a.id = l.account_id
				JOIN payment_events pe ON pe.id = le.source_id
				WHERE le.org_id = ? AND le.currency = ? AND le.source_type = ? AND a.code = ?
				GROUP BY 1
			) s ON s.invoice_id_text = i.id::text
			LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = i.id AND ipt.revoked_at IS NULL
			LEFT JOIN billing_operation_assignments boa 
				ON boa.org_id = ? AND boa.entity_type = 'invoice' AND boa.entity_id = i.id 
				AND boa.status IN ('assigned', 'in_progress')
			WHERE i.org_id = ?
				AND i.status = 'FINALIZED'
				AND i.voided_at IS NULL
				AND i.paid_at IS NULL
				AND i.currency = ?
				AND i.due_at IS NOT NULL
				AND i.due_at < ?
				AND GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) > 0
				AND boa.id IS NULL  -- No active assignment
		),
		risky_customers AS (
			SELECT
				'customer' AS entity_type,
				c.id::text AS entity_id,
				c.name AS entity_name,
				'high_exposure' AS risk_category,
				t.outstanding AS amount_due,
				oo.due_at,
				EXTRACT(EPOCH FROM (? - oo.due_at)) / 86400 AS days_overdue,
				NULL::timestamp AS last_attempt,
				ipt.token_hash,
				(t.outstanding / 10000)::int AS risk_score
			FROM (
				SELECT customer_id, SUM(outstanding) AS outstanding
				FROM (
					SELECT
						i.customer_id,
						GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
					FROM invoices i
					LEFT JOIN (
						SELECT
							(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
							SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
						FROM ledger_entries le
						JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
						JOIN ledger_accounts a ON a.id = l.account_id
						JOIN payment_events pe ON pe.id = le.source_id
						WHERE le.org_id = ? AND le.currency = ? AND le.source_type = ? AND a.code = ?
						GROUP BY 1
					) s ON s.invoice_id_text = i.id::text
					WHERE i.org_id = ? AND i.status = 'FINALIZED' AND i.voided_at IS NULL AND i.currency = ?
				) inv
				WHERE outstanding > 0
				GROUP BY customer_id
			) t
			JOIN customers c ON c.id = t.customer_id
			LEFT JOIN (
				SELECT DISTINCT ON (customer_id)
					customer_id, due_at
				FROM (
					SELECT
						i.customer_id,
						i.due_at
					FROM invoices i
					LEFT JOIN (
						SELECT
							(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
							SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
						FROM ledger_entries le
						JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
						JOIN ledger_accounts a ON a.id = l.account_id
						JOIN payment_events pe ON pe.id = le.source_id
						WHERE le.org_id = ? AND le.currency = ? AND le.source_type = ? AND a.code = ?
						GROUP BY 1
					) s ON s.invoice_id_text = i.id::text
					WHERE i.org_id = ?
						AND i.status = 'FINALIZED'
						AND i.voided_at IS NULL
						AND i.currency = ?
						AND GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) > 0
						AND i.due_at IS NOT NULL
						AND i.due_at < ?
				) inv
				ORDER BY customer_id, due_at ASC
			) oo ON oo.customer_id = t.customer_id
			LEFT JOIN invoice_public_tokens ipt ON ipt.invoice_id = (
				SELECT id FROM invoices WHERE customer_id = c.id AND due_at = oo.due_at LIMIT 1
			) AND ipt.revoked_at IS NULL
			LEFT JOIN billing_operation_assignments boa 
				ON boa.org_id = ? AND boa.entity_type = 'customer' AND boa.entity_id = c.id 
				AND boa.status IN ('assigned', 'in_progress')
			WHERE c.org_id = ?
				AND t.outstanding >= 100000  -- High exposure threshold
				AND boa.id IS NULL  -- No active assignment
		)
		SELECT * FROM (
			SELECT * FROM risky_invoices
			UNION ALL
			SELECT * FROM risky_customers
		) combined
		ORDER BY risk_score DESC, days_overdue DESC
		LIMIT ?`

	currency, err := r.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return nil, err
	}

	var rows []billingopsdomain.InboxRow
	if err := r.db.WithContext(ctx).Raw(
		query,
		now, now,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, orgID, currency, now,
		now,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency, now,
		orgID, orgID,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) ListMyWorkItems(
	ctx context.Context,
	orgID snowflake.ID,
	userID string,
	limit int,
	now time.Time,
) ([]billingopsdomain.MyWorkRow, error) {
	query := `
		SELECT
			boa.id::text AS assignment_id,
			boa.entity_type,
			boa.entity_id::text AS entity_id,
			boa.snapshot_metadata,
			boa.assigned_at,
			boa.status,
			boa.last_action_at,
			-- Current state from billing entities (optional)
			CASE
				WHEN boa.entity_type = 'invoice' THEN COALESCE(i.invoice_number::text, i.id::text)
				WHEN boa.entity_type = 'customer' THEN c.name
			END AS entity_name,
			CASE
				WHEN boa.entity_type = 'invoice' THEN c_inv.name
				WHEN boa.entity_type = 'customer' THEN c.name
			END AS customer_name,
			CASE
				WHEN boa.entity_type = 'invoice' THEN c_inv.email
				WHEN boa.entity_type = 'customer' THEN c.email
			END AS customer_email,
			CASE
				WHEN boa.entity_type = 'invoice' THEN i.invoice_number::text
				ELSE NULL
			END AS invoice_number,
			CASE
				WHEN boa.entity_type = 'invoice' THEN GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0)
				WHEN boa.entity_type = 'customer' THEN t.outstanding
			END AS current_amount_due,
			CASE
				WHEN boa.entity_type = 'invoice' AND i.due_at IS NOT NULL 
					THEN EXTRACT(EPOCH FROM (? - i.due_at)) / 86400
				WHEN boa.entity_type = 'customer' AND oo.due_at IS NOT NULL 
					THEN EXTRACT(EPOCH FROM (? - oo.due_at)) / 86400
			END AS current_days_overdue,
			CASE
				WHEN boa.entity_type = 'invoice' THEN ipt_inv.token_hash
				WHEN boa.entity_type = 'customer' THEN ipt_cust.token_hash
			END AS token_hash
		FROM billing_operation_assignments boa
		LEFT JOIN invoices i ON boa.entity_type = 'invoice' AND boa.entity_id = i.id
		LEFT JOIN customers c ON boa.entity_type = 'customer' AND boa.entity_id = c.id
		LEFT JOIN customers c_inv ON boa.entity_type = 'invoice' AND i.customer_id = c_inv.id
		LEFT JOIN (
			SELECT
				(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
				SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.org_id = ? AND le.currency = ? AND le.source_type = ? AND a.code = ?
			GROUP BY 1
		) s ON s.invoice_id_text = i.id::text
		LEFT JOIN (
			SELECT customer_id, SUM(outstanding) AS outstanding
			FROM (
				SELECT
					i.customer_id,
					GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding
				FROM invoices i
				LEFT JOIN (
					SELECT
						(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
						SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
					FROM ledger_entries le
					JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
					JOIN ledger_accounts a ON a.id = l.account_id
					JOIN payment_events pe ON pe.id = le.source_id
					WHERE le.org_id = ? AND le.currency = ? AND le.source_type = ? AND a.code = ?
					GROUP BY 1
				) s ON s.invoice_id_text = i.id::text
				WHERE i.org_id = ? AND i.status = 'FINALIZED' AND i.voided_at IS NULL AND i.currency = ?
			) inv
			WHERE outstanding > 0
			GROUP BY customer_id
		) t ON boa.entity_type = 'customer' AND t.customer_id = boa.entity_id
		LEFT JOIN (
			SELECT DISTINCT ON (customer_id)
				customer_id, due_at
			FROM (
				SELECT i.customer_id, i.due_at
				FROM invoices i
				LEFT JOIN (
					SELECT
						(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
						SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
					FROM ledger_entries le
					JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
					JOIN ledger_accounts a ON a.id = l.account_id
					JOIN payment_events pe ON pe.id = le.source_id
					WHERE le.org_id = ? AND le.currency = ? AND le.source_type = ? AND a.code = ?
					GROUP BY 1
				) s ON s.invoice_id_text = i.id::text
				WHERE i.org_id = ? AND i.status = 'FINALIZED' AND i.voided_at IS NULL AND i.currency = ?
					AND GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) > 0
					AND i.due_at IS NOT NULL AND i.due_at < ?
			) inv
			ORDER BY customer_id, due_at ASC
		) oo ON boa.entity_type = 'customer' AND oo.customer_id = boa.entity_id
		LEFT JOIN invoice_public_tokens ipt_inv ON boa.entity_type = 'invoice' AND ipt_inv.invoice_id = i.id AND ipt_inv.revoked_at IS NULL
		LEFT JOIN invoice_public_tokens ipt_cust ON boa.entity_type = 'customer' AND ipt_cust.invoice_id = (
			SELECT id FROM invoices WHERE customer_id = c.id AND due_at = oo.due_at LIMIT 1
		) AND ipt_cust.revoked_at IS NULL
		WHERE boa.org_id = ?
			AND boa.assigned_to = ?
			AND boa.status IN ('assigned', 'in_progress')
		ORDER BY boa.assigned_at ASC
		LIMIT ?`

	currency, err := r.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return nil, err
	}

	var rows []billingopsdomain.MyWorkRow
	if err := r.db.WithContext(ctx).Raw(
		query,
		now, now,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency, now,
		orgID, userID,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) ListRecentlyResolvedItems(
	ctx context.Context,
	orgID snowflake.ID,
	userID string,
	limit int,
	since time.Time,
) ([]billingopsdomain.ResolvedRow, error) {
	query := `
		SELECT
			boa.id::text AS assignment_id,
			boa.entity_type,
			boa.entity_id::text AS entity_id,
			boa.snapshot_metadata,
			boa.status,
			boa.resolved_at,
			boa.resolved_by,
			boa.release_reason,
			boa.assigned_at
		FROM billing_operation_assignments boa
		WHERE boa.org_id = ?
			AND boa.assigned_to = ?
			AND boa.status IN ('resolved', 'released', 'escalated')
			AND boa.resolved_at > ?
		ORDER BY boa.resolved_at DESC
		LIMIT ?`

	var rows []billingopsdomain.ResolvedRow
	if err := r.db.WithContext(ctx).Raw(
		query,
		orgID, userID, since,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) GetTeamViewStats(
	ctx context.Context,
	orgID snowflake.ID,
	now time.Time,
) ([]billingopsdomain.TeamRow, error) {
	query := `
		SELECT
			boa.assigned_to AS user_id,
			COUNT(*) AS active_assignments,
			AVG(EXTRACT(EPOCH FROM (? - boa.assigned_at)) / 60)::int AS avg_assignment_age_minutes,
			SUM(
				CASE
					WHEN boa.snapshot_metadata->>'amount_due' IS NOT NULL 
						THEN (boa.snapshot_metadata->>'amount_due')::numeric::bigint
					ELSE 0
				END
			) AS total_exposure_owned,
			SUM(CASE WHEN boa.status = 'escalated' THEN 1 ELSE 0 END) AS escalation_count
		FROM billing_operation_assignments boa
		WHERE boa.org_id = ?
			AND boa.status IN ('assigned', 'in_progress', 'escalated')
		GROUP BY boa.assigned_to
		ORDER BY boa.assigned_to ASC`

	var rows []billingopsdomain.TeamRow
	if err := r.db.WithContext(ctx).Raw(
		query,
		now, orgID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) ListInvoicePayments(
	ctx context.Context,
	orgID, invoiceID snowflake.ID,
) ([]billingopsdomain.PaymentRow, error) {
	query := `
		SELECT
			pe.provider_payment_id,
			pe.provider,
			pe.event_type,
			pe.received_at,
			pe.currency,
			pe.payload
		FROM payment_events pe
		WHERE pe.org_id = ?
		  AND pe.payload -> 'data' -> 'object' -> 'metadata' ->> 'invoice_id' = ?
		ORDER BY pe.received_at DESC`

	var rows []billingopsdomain.PaymentRow
	if err := r.db.WithContext(ctx).Raw(query, orgID, invoiceID.String()).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) GetExposureStats(
	ctx context.Context,
	orgID snowflake.ID,
	now time.Time,
) (billingopsdomain.ExposureStatsRow, error) {
	query := `
		SELECT
			COALESCE(SUM(outstanding), 0) AS total_exposure,
			COALESCE(SUM(CASE WHEN days_overdue <= 0 THEN outstanding ELSE 0 END), 0) AS current_amount,
			COALESCE(SUM(CASE WHEN days_overdue > 0 AND days_overdue <= 30 THEN outstanding ELSE 0 END), 0) AS bucket_0_30,
			COALESCE(SUM(CASE WHEN days_overdue > 30 AND days_overdue <= 60 THEN outstanding ELSE 0 END), 0) AS bucket_31_60,
			COALESCE(SUM(CASE WHEN days_overdue > 60 AND days_overdue <= 90 THEN outstanding ELSE 0 END), 0) AS bucket_61_90,
			COALESCE(SUM(CASE WHEN days_overdue > 90 THEN outstanding ELSE 0 END), 0) AS bucket_90_plus,
			COUNT(CASE WHEN days_overdue > 0 THEN 1 END) AS overdue_count
		FROM (
			SELECT
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding,
				EXTRACT(EPOCH FROM (? - i.due_at)) / 86400 AS days_overdue
			FROM invoices i
			LEFT JOIN (
				SELECT
					(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
					SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
				FROM ledger_entries le
				JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
				JOIN ledger_accounts a ON a.id = l.account_id
				JOIN payment_events pe ON pe.id = le.source_id
				WHERE le.org_id = ? AND le.currency = ? AND le.source_type = ? AND a.code = ?
				GROUP BY 1
			) s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ?
				AND i.status = 'FINALIZED'
				AND i.voided_at IS NULL
				AND i.paid_at IS NULL
				AND i.currency = ?
				AND i.due_at IS NOT NULL
		) inv
		WHERE outstanding > 0`

	currency, err := r.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return billingopsdomain.ExposureStatsRow{}, err
	}

	var stats billingopsdomain.ExposureStatsRow
	if err := r.db.WithContext(ctx).Raw(
		query,
		now,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency,
	).Scan(&stats).Error; err != nil {
		return billingopsdomain.ExposureStatsRow{}, err
	}
	return stats, nil
}

func (r *RepositoryImpl) ListTopHighExposure(
	ctx context.Context,
	orgID snowflake.ID,
	now time.Time,
) ([]billingopsdomain.TopCustomerExposureRow, error) {
	query := `
		SELECT
			c.name AS entity_name,
			SUM(outstanding) AS amount_due,
			(SUM(outstanding) / 10000)::int AS risk_score,
			MAX(days_overdue) AS days_overdue
		FROM (
			SELECT
				i.customer_id,
				GREATEST(i.subtotal_amount - COALESCE(s.settled_amount, 0), 0) AS outstanding,
				(EXTRACT(EPOCH FROM (? - i.due_at)) / 86400)::int AS days_overdue
			FROM invoices i
			LEFT JOIN (
				SELECT
					(pe.payload #>> '{data,object,metadata,invoice_id}') AS invoice_id_text,
					SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS settled_amount
				FROM ledger_entries le
				JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
				JOIN ledger_accounts a ON a.id = l.account_id
				JOIN payment_events pe ON pe.id = le.source_id
				WHERE le.org_id = ? AND le.currency = ? AND le.source_type = ? AND a.code = ?
				GROUP BY 1
			) s ON s.invoice_id_text = i.id::text
			WHERE i.org_id = ? AND i.status = 'FINALIZED' AND i.voided_at IS NULL AND i.currency = ?
		) inv
		JOIN customers c ON c.id = inv.customer_id
		WHERE outstanding > 0
		GROUP BY c.id, c.name
		ORDER BY amount_due DESC
		LIMIT 5`

	currency, err := r.FetchOrgCurrency(ctx, orgID)
	if err != nil {
		return nil, err
	}

	var rows []billingopsdomain.TopCustomerExposureRow
	if err := r.db.WithContext(ctx).Raw(
		query,
		now,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *RepositoryImpl) ListBillingAssignmentsForPerformance(
	ctx context.Context,
	orgID snowflake.ID,
	userID string,
	start, end time.Time,
) ([]billingopsdomain.BillingAssignmentRow, error) {
	var assignments []billingopsdomain.BillingAssignmentRow
	if err := r.db.WithContext(ctx).Table("billing_operation_assignments").
		Where("org_id = ? AND assigned_to = ? AND assigned_at >= ? AND assigned_at < ?", orgID, userID, start, end).
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}


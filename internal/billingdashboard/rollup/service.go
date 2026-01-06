package rollup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/events"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	ledgerdomain "github.com/smallbiznis/valora/internal/ledger/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	rebuildStatusPending    = "pending"
	rebuildStatusProcessing = "processing"
	rebuildStatusCompleted  = "completed"
	rebuildStatusFailed     = "failed"

	rebuildBatchSize = 500
)

type Params struct {
	fx.In

	DB    *gorm.DB
	Log   *zap.Logger
	GenID *snowflake.Node
}

type Service struct {
	db    *gorm.DB
	log   *zap.Logger
	genID *snowflake.Node
}

func NewService(p Params) *Service {
	return &Service{
		db:    p.DB,
		log:   p.Log.Named("billingdashboard.rollup"),
		genID: p.GenID,
	}
}

type eventRow struct {
	ID        snowflake.ID      `gorm:"column:id"`
	OrgID     snowflake.ID      `gorm:"column:org_id"`
	EventType string            `gorm:"column:event_type"`
	Payload   datatypes.JSONMap `gorm:"column:payload"`
	Published bool              `gorm:"column:published"`
}

// ProcessPending consumes billing events and updates snapshot tables.
func (s *Service) ProcessPending(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 50
	}

	types := []string{
		events.EventLedgerEntryCreated,
		events.EventInvoiceFinalized,
		events.EventInvoiceVoided,
		events.EventPaymentSettled,
		events.EventRefundSettled,
		events.EventDisputeWithdrawn,
		events.EventDisputeReinstated,
	}

	var rows []eventRow
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id, org_id, event_type, payload, published
		 FROM billing_events
		 WHERE published = false AND event_type IN ?
		 ORDER BY created_at ASC
		 LIMIT ?`,
		types,
		limit,
	).Scan(&rows).Error; err != nil {
		return err
	}

	var jobErr error
	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := s.processEvent(ctx, row); err != nil {
			jobErr = errors.Join(jobErr, err)
			s.log.Warn("failed to process billing event", zap.Error(err), zap.String("event_id", row.ID.String()))
		}
	}

	return jobErr
}

func (s *Service) processEvent(ctx context.Context, row eventRow) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked eventRow
		if err := tx.WithContext(ctx).Raw(
			`SELECT id, org_id, event_type, payload, published
			 FROM billing_events
			 WHERE id = ?
			 FOR UPDATE`,
			row.ID,
		).Scan(&locked).Error; err != nil {
			return err
		}
		if locked.ID == 0 || locked.Published {
			return nil
		}
		if err := s.applyEvent(ctx, tx, locked); err != nil {
			return err
		}
		now := time.Now().UTC()
		return tx.WithContext(ctx).Exec(
			`UPDATE billing_events SET published = true, published_at = ? WHERE id = ?`,
			now,
			locked.ID,
		).Error
	})
}

func (s *Service) applyEvent(ctx context.Context, tx *gorm.DB, row eventRow) error {
	switch strings.TrimSpace(row.EventType) {
	case events.EventLedgerEntryCreated,
		events.EventPaymentSettled,
		events.EventRefundSettled,
		events.EventDisputeWithdrawn,
		events.EventDisputeReinstated:
		entryID, err := parseSnowflakePayload(row.Payload, "ledger_entry_id")
		if err != nil {
			return err
		}
		return s.applyLedgerEntry(ctx, tx, entryID)
	case events.EventInvoiceFinalized:
		cycleID, err := parseSnowflakePayload(row.Payload, "billing_cycle_id")
		if err != nil {
			return err
		}
		return s.applyInvoiceCountDelta(ctx, tx, cycleID, 1)
	case events.EventInvoiceVoided:
		cycleID, err := parseSnowflakePayload(row.Payload, "billing_cycle_id")
		if err != nil {
			return err
		}
		return s.applyInvoiceCountDelta(ctx, tx, cycleID, -1)
	default:
		return nil
	}
}

func (s *Service) applyLedgerEntry(ctx context.Context, tx *gorm.DB, entryID snowflake.ID) error {
	if entryID == 0 {
		return errors.New("invalid_ledger_entry_id")
	}
	if err := s.applyCustomerBalanceDelta(ctx, tx, entryID); err != nil {
		return err
	}
	return s.applyRevenueDelta(ctx, tx, entryID)
}

type balanceDeltaRow struct {
	OrgID      snowflake.ID `gorm:"column:org_id"`
	CustomerID snowflake.ID `gorm:"column:customer_id"`
	Currency   string       `gorm:"column:currency"`
	Delta      int64        `gorm:"column:delta"`
}

func (s *Service) applyCustomerBalanceDelta(
	ctx context.Context,
	tx *gorm.DB,
	entryID snowflake.ID,
) error {

	var rows []balanceDeltaRow

	if err := tx.WithContext(ctx).Raw(
		`
		WITH ar_delta AS (

			-- Billing cycle & adjustment (subscription-scoped)
			SELECT
				le.org_id,
				s.customer_id,
				le.currency,
				SUM(CASE l.direction WHEN 'debit' THEN l.amount ELSE -l.amount END) AS delta
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN billing_cycles bc ON bc.id = le.source_id
			JOIN subscriptions s ON s.id = bc.subscription_id
			WHERE le.id = ?
			  AND le.source_type IN (?, ?)
			  AND a.code = ?
			GROUP BY le.org_id, s.customer_id, le.currency

			UNION ALL

			-- Payments, fees, refunds (payment-scoped)
			SELECT
				le.org_id,
				pe.customer_id,
				le.currency,
				SUM(CASE l.direction WHEN 'debit' THEN l.amount ELSE -l.amount END) AS delta
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_events pe ON pe.id = le.source_id
			WHERE le.id = ?
			  AND le.source_type IN (?, ?, ?)
			  AND a.code = ?
			GROUP BY le.org_id, pe.customer_id, le.currency

			UNION ALL

			-- Disputes (economic impact only)
			SELECT
				le.org_id,
				pd.customer_id,
				le.currency,
				SUM(CASE l.direction WHEN 'debit' THEN l.amount ELSE -l.amount END) AS delta
			FROM ledger_entries le
			JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
			JOIN ledger_accounts a ON a.id = l.account_id
			JOIN payment_disputes pd ON pd.id = le.source_id
			WHERE le.id = ?
			  AND le.source_type IN (?, ?, ?)
			  AND a.code = ?
			GROUP BY le.org_id, pd.customer_id, le.currency
		)

		SELECT org_id, customer_id, currency, SUM(delta) AS delta
		FROM ar_delta
		GROUP BY org_id, customer_id, currency
		`,
		// billing / adjustment
		entryID,
		ledgerdomain.SourceTypeBillingCycle,
		ledgerdomain.SourceTypeAdjustment,
		ledgerdomain.AccountCodeAccountsReceivable,

		// payment scoped
		entryID,
		ledgerdomain.SourceTypePayment,
		ledgerdomain.SourceTypePaymentFee,
		ledgerdomain.SourceTypeRefund,
		ledgerdomain.AccountCodeAccountsReceivable,

		// disputes
		entryID,
		ledgerdomain.SourceTypeDisputeHold,
		ledgerdomain.SourceTypeDisputeLoss,
		ledgerdomain.SourceTypeDisputeWin,
		ledgerdomain.AccountCodeAccountsReceivable,
	).Scan(&rows).Error; err != nil {
		return err
	}

	now := time.Now().UTC()

	for _, row := range rows {
		currency := strings.ToUpper(strings.TrimSpace(row.Currency))

		if err := tx.WithContext(ctx).Exec(
			`
			INSERT INTO customer_balances (org_id, customer_id, currency, balance, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (org_id, customer_id, currency)
			DO UPDATE SET
				balance = customer_balances.balance + EXCLUDED.balance,
				updated_at = EXCLUDED.updated_at
			`,
			row.OrgID,
			row.CustomerID,
			currency,
			row.Delta,
			now,
		).Error; err != nil {
			return err
		}
	}

	return nil
}

type revenueDeltaRow struct {
	OrgID          snowflake.ID `gorm:"column:org_id"`
	BillingCycleID snowflake.ID `gorm:"column:billing_cycle_id"`
	PeriodStart    time.Time    `gorm:"column:period_start"`
	Status         string       `gorm:"column:status"`
	Delta          int64        `gorm:"column:delta"`
}

func (s *Service) applyRevenueDelta(
	ctx context.Context,
	tx *gorm.DB,
	entryID snowflake.ID,
) error {

	var rows []revenueDeltaRow

	if err := tx.WithContext(ctx).Raw(
		`
		SELECT
			le.org_id AS org_id,
			le.source_id AS billing_cycle_id,
			bc.period_start AS period_start,
			bc.status AS status,
			SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END) AS delta
		FROM ledger_entries le
		JOIN ledger_entry_lines l ON l.ledger_entry_id = le.id
		JOIN ledger_accounts a ON a.id = l.account_id
		JOIN billing_cycles bc ON bc.id = le.source_id
		WHERE le.id = ?
		  AND le.source_type IN (?, ?)
		  AND a.code IN (?, ?)
		GROUP BY
			le.org_id,
			le.source_id,
			bc.period_start,
			bc.status
		`,
		entryID,
		ledgerdomain.SourceTypeBillingCycle,
		ledgerdomain.SourceTypeAdjustment,
		ledgerdomain.AccountCodeRevenueFlat,
		ledgerdomain.AccountCodeRevenueUsage,
	).Scan(&rows).Error; err != nil {
		return err
	}

	now := time.Now().UTC()

	for _, row := range rows {
		if err := tx.WithContext(ctx).Exec(
			`
			INSERT INTO billing_cycle_stats (
				billing_cycle_id,
				org_id,
				period_start,
				status,
				total_revenue,
				invoice_count,
				updated_at
			)
			VALUES (?, ?, ?, ?, ?, 0, ?)
			ON CONFLICT (billing_cycle_id)
			DO UPDATE SET
				total_revenue = billing_cycle_stats.total_revenue + EXCLUDED.total_revenue,
				period_start = EXCLUDED.period_start,
				status = EXCLUDED.status,
				updated_at = EXCLUDED.updated_at
			`,
			row.BillingCycleID,
			row.OrgID,
			row.PeriodStart,
			strings.TrimSpace(row.Status),
			row.Delta,
			now,
		).Error; err != nil {
			return err
		}
	}

	return nil
}

type cycleMetaRow struct {
	OrgID       snowflake.ID `gorm:"column:org_id"`
	PeriodStart time.Time    `gorm:"column:period_start"`
	Status      string       `gorm:"column:status"`
}

func (s *Service) applyInvoiceCountDelta(ctx context.Context, tx *gorm.DB, cycleID snowflake.ID, delta int64) error {
	if cycleID == 0 {
		return errors.New("invalid_billing_cycle_id")
	}
	var meta cycleMetaRow
	if err := tx.WithContext(ctx).Raw(
		`SELECT org_id, period_start, status
		 FROM billing_cycles
		 WHERE id = ?`,
		cycleID,
	).Scan(&meta).Error; err != nil {
		return err
	}
	if meta.OrgID == 0 {
		return errors.New("billing_cycle_not_found")
	}

	now := time.Now().UTC()
	return tx.WithContext(ctx).Exec(
		`INSERT INTO billing_cycle_stats (billing_cycle_id, org_id, period_start, status, total_revenue, invoice_count, updated_at)
		 VALUES (?, ?, ?, ?, 0, ?, ?)
		 ON CONFLICT (billing_cycle_id)
		 DO UPDATE SET invoice_count = billing_cycle_stats.invoice_count + EXCLUDED.invoice_count,
		               period_start = EXCLUDED.period_start,
		               status = EXCLUDED.status,
		               updated_at = EXCLUDED.updated_at`,
		cycleID,
		meta.OrgID,
		meta.PeriodStart,
		strings.TrimSpace(meta.Status),
		delta,
		now,
	).Error
}

// RebuildRequest describes the scope of a snapshot rebuild.
type RebuildRequest struct {
	OrgID          *snowflake.ID
	BillingCycleID *snowflake.ID
}

// EnqueueRebuild stores a rebuild request for async processing.
func (s *Service) EnqueueRebuild(ctx context.Context, req RebuildRequest) (string, error) {
	if s.genID == nil {
		return "", errors.New("missing_id_generator")
	}
	id := s.genID.Generate()
	now := time.Now().UTC()
	var orgValue any
	if req.OrgID != nil && *req.OrgID != 0 {
		orgValue = *req.OrgID
	}
	var cycleValue any
	if req.BillingCycleID != nil && *req.BillingCycleID != 0 {
		cycleValue = *req.BillingCycleID
	}

	if err := s.db.WithContext(ctx).Exec(
		`INSERT INTO billing_snapshot_rebuild_requests (id, org_id, billing_cycle_id, status, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id,
		orgValue,
		cycleValue,
		rebuildStatusPending,
		now,
	).Error; err != nil {
		return "", err
	}
	return id.String(), nil
}

// ProcessRebuildRequests processes pending snapshot rebuild requests.
func (s *Service) ProcessRebuildRequests(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 10
	}

	var rows []rebuildRequestRow
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id, org_id, billing_cycle_id
		 FROM billing_snapshot_rebuild_requests
		 WHERE status = ?
		 ORDER BY created_at ASC
		 LIMIT ?`,
		rebuildStatusPending,
		limit,
	).Scan(&rows).Error; err != nil {
		return err
	}

	var jobErr error
	for _, row := range rows {
		if ctx.Err() == nil {
			return ctx.Err()
		}

		if err := s.processRebuildRequest(ctx, row); err != nil {
			jobErr = errors.Join(jobErr, err)
			s.log.Warn("failed to rebuild billing snapshots", zap.Error(err), zap.String("request_id", row.ID.String()))
		}
	}

	return jobErr
}

type rebuildRequestRow struct {
	ID             snowflake.ID  `gorm:"column:id"`
	OrgID          *snowflake.ID `gorm:"column:org_id"`
	BillingCycleID *snowflake.ID `gorm:"column:billing_cycle_id"`
}

func (s *Service) processRebuildRequest(ctx context.Context, row rebuildRequestRow) error {
	rebuildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	now := time.Now().UTC()
	result := s.db.WithContext(rebuildCtx).Exec(
		`UPDATE billing_snapshot_rebuild_requests
		 SET status = ?, started_at = ?
		 WHERE id = ? AND status = ?`,
		rebuildStatusProcessing,
		now,
		row.ID,
		rebuildStatusPending,
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}

	req := RebuildRequest{OrgID: row.OrgID, BillingCycleID: row.BillingCycleID}
	err := s.RebuildSnapshots(rebuildCtx, req)
	completedAt := time.Now().UTC()
	if err != nil {
		return s.db.WithContext(rebuildCtx).Exec(
			`UPDATE billing_snapshot_rebuild_requests
			 SET status = ?, error = ?, completed_at = ?
			 WHERE id = ?`,
			rebuildStatusFailed,
			errorSummary(err),
			completedAt,
			row.ID,
		).Error
	}

	return s.db.WithContext(rebuildCtx).Exec(
		`UPDATE billing_snapshot_rebuild_requests
		 SET status = ?, completed_at = ?
		 WHERE id = ?`,
		rebuildStatusCompleted,
		completedAt,
		row.ID,
	).Error
}

// RebuildSnapshots truncates and replays snapshots from ledger entries.
func (s *Service) RebuildSnapshots(ctx context.Context, req RebuildRequest) error {
	scopeOrgID, err := s.resolveOrgScope(ctx, req)
	if err != nil {
		return err
	}

	if err := s.clearSnapshots(ctx, scopeOrgID); err != nil {
		return err
	}

	if err := s.seedCycleStats(ctx, scopeOrgID); err != nil {
		return err
	}

	if err := s.replayLedgerEntries(ctx, scopeOrgID); err != nil {
		return err
	}

	return s.recomputeInvoiceCounts(ctx, scopeOrgID)
}

func (s *Service) resolveOrgScope(ctx context.Context, req RebuildRequest) (*snowflake.ID, error) {
	if req.BillingCycleID != nil && *req.BillingCycleID != 0 {
		var orgID snowflake.ID
		if err := s.db.WithContext(ctx).Raw(
			`SELECT org_id FROM billing_cycles WHERE id = ?`,
			*req.BillingCycleID,
		).Scan(&orgID).Error; err != nil {
			return nil, err
		}
		if orgID == 0 {
			return nil, errors.New("billing_cycle_not_found")
		}
		if req.OrgID != nil && *req.OrgID != 0 && *req.OrgID != orgID {
			return nil, errors.New("billing_cycle_org_mismatch")
		}
		return &orgID, nil
	}
	if req.OrgID != nil && *req.OrgID != 0 {
		return req.OrgID, nil
	}
	return nil, nil
}

func (s *Service) clearSnapshots(ctx context.Context, orgID *snowflake.ID) error {
	if orgID == nil {
		if err := s.db.WithContext(ctx).Exec(`TRUNCATE customer_balances, billing_cycle_stats`).Error; err == nil {
			return nil
		}
		if err := s.db.WithContext(ctx).Exec(`DELETE FROM customer_balances`).Error; err != nil {
			return err
		}
		return s.db.WithContext(ctx).Exec(`DELETE FROM billing_cycle_stats`).Error
	}

	if err := s.db.WithContext(ctx).Exec(
		`DELETE FROM customer_balances WHERE org_id = ?`,
		*orgID,
	).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Exec(
		`DELETE FROM billing_cycle_stats WHERE org_id = ?`,
		*orgID,
	).Error
}

func (s *Service) seedCycleStats(ctx context.Context, orgID *snowflake.ID) error {
	args := []any{time.Now().UTC()}
	query := `INSERT INTO billing_cycle_stats (billing_cycle_id, org_id, period_start, status, total_revenue, invoice_count, updated_at)
		SELECT id, org_id, period_start, status, 0, 0, ?
		FROM billing_cycles`
	if orgID != nil {
		query += " WHERE org_id = ?"
		args = append(args, *orgID)
	}
	query += ` ON CONFLICT (billing_cycle_id)
		DO UPDATE SET period_start = EXCLUDED.period_start,
		               status = EXCLUDED.status,
		               updated_at = EXCLUDED.updated_at`

	return s.db.WithContext(ctx).Exec(query, args...).Error
}

type ledgerEntryRow struct {
	ID         snowflake.ID `gorm:"column:id"`
	OccurredAt time.Time    `gorm:"column:occurred_at"`
}

func (s *Service) replayLedgerEntries(ctx context.Context, orgID *snowflake.ID) error {
	lastOccurred := time.Time{}
	lastID := snowflake.ID(0)

	for {

		if ctx.Err() != nil {
			return ctx.Err()
		}

		args := []any{lastOccurred, lastOccurred, lastID}
		query := `SELECT id, occurred_at
			FROM ledger_entries
			WHERE (occurred_at > ? OR (occurred_at = ? AND id > ?))`
		if orgID != nil {
			query += " AND org_id = ?"
			args = append(args, *orgID)
		}
		query += " ORDER BY occurred_at ASC, id ASC LIMIT ?"
		args = append(args, rebuildBatchSize)

		var rows []ledgerEntryRow
		if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		for _, row := range rows {
			if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				return s.applyLedgerEntry(ctx, tx, row.ID)
			}); err != nil {
				return err
			}
			lastOccurred = row.OccurredAt
			lastID = row.ID
		}
	}
}

type invoiceCountRow struct {
	BillingCycleID snowflake.ID `gorm:"column:billing_cycle_id"`
	InvoiceCount   int64        `gorm:"column:invoice_count"`
}

func (s *Service) recomputeInvoiceCounts(ctx context.Context, orgID *snowflake.ID) error {
	args := []any{invoicedomain.InvoiceStatusFinalized}
	query := `SELECT billing_cycle_id, COUNT(1) AS invoice_count
		FROM invoices
		WHERE status = ?`
	if orgID != nil {
		query += " AND org_id = ?"
		args = append(args, *orgID)
	}
	query += " GROUP BY billing_cycle_id"

	var rows []invoiceCountRow
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		if err := s.setInvoiceCount(ctx, row.BillingCycleID, row.InvoiceCount); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) setInvoiceCount(ctx context.Context, cycleID snowflake.ID, count int64) error {
	if cycleID == 0 {
		return errors.New("invalid_billing_cycle_id")
	}

	var meta cycleMetaRow
	if err := s.db.WithContext(ctx).Raw(
		`SELECT org_id, period_start, status
		 FROM billing_cycles
		 WHERE id = ?`,
		cycleID,
	).Scan(&meta).Error; err != nil {
		return err
	}
	if meta.OrgID == 0 {
		return errors.New("billing_cycle_not_found")
	}

	now := time.Now().UTC()
	return s.db.WithContext(ctx).Exec(
		`INSERT INTO billing_cycle_stats (billing_cycle_id, org_id, period_start, status, total_revenue, invoice_count, updated_at)
		 VALUES (?, ?, ?, ?, 0, ?, ?)
		 ON CONFLICT (billing_cycle_id)
		 DO UPDATE SET invoice_count = EXCLUDED.invoice_count,
		               period_start = EXCLUDED.period_start,
		               status = EXCLUDED.status,
		               updated_at = EXCLUDED.updated_at`,
		cycleID,
		meta.OrgID,
		meta.PeriodStart,
		strings.TrimSpace(meta.Status),
		count,
		now,
	).Error
}

func parseSnowflakePayload(payload datatypes.JSONMap, key string) (snowflake.ID, error) {
	value, ok := payload[key]
	if !ok {
		return 0, fmt.Errorf("missing_payload_%s", key)
	}
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, fmt.Errorf("missing_payload_%s", key)
		}
		return snowflake.ParseString(trimmed)
	case float64:
		return snowflake.ID(int64(typed)), nil
	case int64:
		return snowflake.ID(typed), nil
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, err
		}
		return snowflake.ID(parsed), nil
	default:
		return 0, fmt.Errorf("invalid_payload_%s", key)
	}
}

func errorSummary(err error) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	if value == "" {
		return "unknown_error"
	}
	if len(value) > 256 {
		return value[:256]
	}
	return value
}

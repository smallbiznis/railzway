package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/smallbiznis/railzway/internal/billingoperations/domain"
	ledgerdomain "github.com/smallbiznis/railzway/internal/ledger/domain"
	"github.com/smallbiznis/railzway/internal/orgcontext"
	"go.uber.org/zap"
	"gorm.io/datatypes"
)

// GetInbox returns unassigned risky items that need attention
// Routing Rule: Entity appears ONLY if it meets billing risk criteria AND has NO active assignment
func (s *Service) GetInbox(ctx context.Context, req domain.InboxRequest) (domain.InboxResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.InboxResponse{}, domain.ErrInvalidOrganization
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 25
	}

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.InboxResponse{}, err
	}

	now := s.clock.Now().UTC()

	// Query for unassigned risky items
	// Combines overdue invoices, failed payments, and high-exposure customers
	// Filters out items with active assignments
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

	type inboxRow struct {
		EntityType   string         `gorm:"column:entity_type"`
		EntityID     string         `gorm:"column:entity_id"`
		EntityName   string         `gorm:"column:entity_name"`
		RiskCategory string         `gorm:"column:risk_category"`
		AmountDue    int64          `gorm:"column:amount_due"`
		DueAt        sql.NullTime   `gorm:"column:due_at"`
		DaysOverdue  float64        `gorm:"column:days_overdue"`
		LastAttempt  sql.NullTime   `gorm:"column:last_attempt"`
		TokenHash    sql.NullString `gorm:"column:token_hash"`
		RiskScore    int            `gorm:"column:risk_score"`
	}

	var rows []inboxRow
	if err := s.db.WithContext(ctx).Raw(
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
		return domain.InboxResponse{}, err
	}

	items := make([]domain.InboxItem, 0, len(rows))
	for _, row := range rows {
		var lastAttempt *time.Time
		if row.LastAttempt.Valid {
			t := row.LastAttempt.Time.UTC()
			lastAttempt = &t
		}

		items = append(items, domain.InboxItem{
			EntityType:   row.EntityType,
			EntityID:     row.EntityID,
			EntityName:   row.EntityName,
			RiskCategory: row.RiskCategory,
			RiskScore:    row.RiskScore,
			AmountDue:    row.AmountDue,
			Currency:     currency,
			DaysOverdue:  int(row.DaysOverdue),
			LastAttempt:  lastAttempt,
			PublicToken:  decryptToken(s.encKey, row.TokenHash.String),
		})
	}

	return domain.InboxResponse{
		Items:    items,
		Currency: currency,
	}, nil
}

// GetMyWork returns tasks currently owned by the logged-in user
// Routing Rule: assigned_to = current_user AND status IN (claimed, in_progress)
// CRITICAL: Never filters by billing state - tasks remain visible until explicitly resolved/released
func (s *Service) GetMyWork(ctx context.Context, userID string, req domain.MyWorkRequest) (domain.MyWorkResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.MyWorkResponse{}, domain.ErrInvalidOrganization
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.MyWorkResponse{}, err
	}

	now := s.clock.Now().UTC()

		// Query assignments (source of truth for My Work)
	// Joins with billing entities to get current state (optional, non-sorting)
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

	type myWorkRow struct {
		AssignmentID       string          `gorm:"column:assignment_id"`
		EntityType         string          `gorm:"column:entity_type"`
		EntityID           string          `gorm:"column:entity_id"`
		SnapshotMetadata   datatypes.JSON  `gorm:"column:snapshot_metadata"`
		AssignedAt         time.Time       `gorm:"column:assigned_at"`
		Status             string          `gorm:"column:status"`
		LastActionAt       sql.NullTime    `gorm:"column:last_action_at"`
		EntityName         sql.NullString  `gorm:"column:entity_name"`
		CustomerName       sql.NullString  `gorm:"column:customer_name"`
		CustomerEmail      sql.NullString  `gorm:"column:customer_email"`
		InvoiceNumber      sql.NullString  `gorm:"column:invoice_number"`

		CurrentAmountDue   sql.NullInt64   `gorm:"column:current_amount_due"`
		CurrentDaysOverdue sql.NullFloat64 `gorm:"column:current_days_overdue"`
		TokenHash          sql.NullString  `gorm:"column:token_hash"`
	}

	var rows []myWorkRow
	if err := s.db.WithContext(ctx).Raw(
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
		return domain.MyWorkResponse{}, err
	}

	items := make([]domain.MyWorkItem, 0, len(rows))
	for _, row := range rows {
		// Parse snapshot metadata
		var snapshot map[string]interface{}
		if len(row.SnapshotMetadata) > 0 {
			if err := json.Unmarshal(row.SnapshotMetadata, &snapshot); err != nil {
				s.log.Warn("failed to unmarshal snapshot metadata", zap.Error(err))
				snapshot = make(map[string]interface{})
			}
		} else {
			snapshot = make(map[string]interface{})
		}

		// Extract snapshot values (stable)
		entityName := row.EntityName.String
		if name, ok := snapshot["entity_name"].(string); ok && name != "" {
			entityName = name
		}

		amountDueAtClaim := int64(0)
		if amt, ok := snapshot["amount_due"].(float64); ok {
			amountDueAtClaim = int64(amt)
		}

		daysOverdueAtClaim := 0
		if days, ok := snapshot["days_overdue"].(float64); ok {
			daysOverdueAtClaim = int(days)
		}

		// Current values (optional)
		var currentAmountDue int64
		if row.CurrentAmountDue.Valid {
			currentAmountDue = row.CurrentAmountDue.Int64
		}

		var currentDaysOverdue int
		if row.CurrentDaysOverdue.Valid {
			currentDaysOverdue = int(row.CurrentDaysOverdue.Float64)
		}

		// Calculate assignment age
		duration := now.Sub(row.AssignedAt)
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		var assignmentAge string
		if hours > 0 {
			assignmentAge = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			assignmentAge = fmt.Sprintf("%dm", minutes)
		}

		var lastActionAt *time.Time
		if row.LastActionAt.Valid {
			t := row.LastActionAt.Time.UTC()
			lastActionAt = &t
		}

		items = append(items, domain.MyWorkItem{
			AssignmentID:       row.AssignmentID,
			EntityType:         row.EntityType,
			EntityID:           row.EntityID,
			EntityName:         entityName,
			CustomerName:       row.CustomerName.String,
			CustomerEmail:      row.CustomerEmail.String,
			InvoiceNumber:      row.InvoiceNumber.String,

			AmountDueAtClaim:   amountDueAtClaim,
			DaysOverdueAtClaim: daysOverdueAtClaim,
			CurrentAmountDue:   currentAmountDue,
			CurrentDaysOverdue: currentDaysOverdue,
			Currency:           currency,
			ClaimedAt:          row.AssignedAt.UTC(),
			AssignmentAge:      assignmentAge,
			Status:             row.Status,
			LastActionAt:       lastActionAt,
			PublicToken:        decryptToken(s.encKey, row.TokenHash.String),
		})
	}

	return domain.MyWorkResponse{
		Items:    items,
		Currency: currency,
	}, nil
}

// GetRecentlyResolved returns completed, released, or escalated work
// Routing Rule: assigned_to = current_user AND status IN (resolved, released, escalated) AND resolved_at > 30 days ago
func (s *Service) GetRecentlyResolved(ctx context.Context, userID string, req domain.RecentlyResolvedRequest) (domain.RecentlyResolvedResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.RecentlyResolvedResponse{}, domain.ErrInvalidOrganization
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	now := s.clock.Now().UTC()
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.RecentlyResolvedResponse{}, err
	}

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

	type resolvedRow struct {
		AssignmentID     string         `gorm:"column:assignment_id"`
		EntityType       string         `gorm:"column:entity_type"`
		EntityID         string         `gorm:"column:entity_id"`
		SnapshotMetadata datatypes.JSON `gorm:"column:snapshot_metadata"`
		Status           string         `gorm:"column:status"`
		ResolvedAt       time.Time      `gorm:"column:resolved_at"`
		ResolvedBy       sql.NullString `gorm:"column:resolved_by"`
		ReleaseReason    sql.NullString `gorm:"column:release_reason"`
		AssignedAt       time.Time      `gorm:"column:assigned_at"`
	}

	var rows []resolvedRow
	if err := s.db.WithContext(ctx).Raw(
		query,
		orgID, userID, thirtyDaysAgo,
		limit,
	).Scan(&rows).Error; err != nil {
		return domain.RecentlyResolvedResponse{}, err
	}

	items := make([]domain.ResolvedItem, 0, len(rows))
	for _, row := range rows {
		// Parse snapshot metadata
		var snapshot map[string]interface{}
		if len(row.SnapshotMetadata) > 0 {
			if err := json.Unmarshal(row.SnapshotMetadata, &snapshot); err != nil {
				s.log.Warn("failed to unmarshal snapshot metadata", zap.Error(err))
				snapshot = make(map[string]interface{})
			}
		} else {
			snapshot = make(map[string]interface{})
		}

		entityName := ""
		if name, ok := snapshot["entity_name"].(string); ok {
			entityName = name
		}

		amountDueAtClaim := int64(0)
		if amt, ok := snapshot["amount_due"].(float64); ok {
			amountDueAtClaim = int64(amt)
		}

		// Calculate duration
		duration := row.ResolvedAt.Sub(row.AssignedAt)
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		var durationStr string
		if hours > 0 {
			durationStr = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			durationStr = fmt.Sprintf("%dm", minutes)
		}

		items = append(items, domain.ResolvedItem{
			AssignmentID:     row.AssignmentID,
			EntityType:       row.EntityType,
			EntityID:         row.EntityID,
			EntityName:       entityName,
			Status:           row.Status,
			ResolvedAt:       row.ResolvedAt.UTC(),
			ResolvedBy:       row.ResolvedBy.String,
			Reason:           row.ReleaseReason.String,
			ClaimedAt:        row.AssignedAt.UTC(),
			Duration:         durationStr,
			AmountDueAtClaim: amountDueAtClaim,
			Currency:         currency,
		})
	}

	return domain.RecentlyResolvedResponse{
		Items: items,
	}, nil
}

// GetTeamView returns operational oversight for managers
// Routing Rule: Manager role only
// Explicitly FORBIDDEN: Leaderboards, ranking by score, best/worst labels
func (s *Service) GetTeamView(ctx context.Context, req domain.TeamViewRequest) (domain.TeamViewResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.TeamViewResponse{}, domain.ErrInvalidOrganization
	}

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.TeamViewResponse{}, err
	}

	now := s.clock.Now().UTC()

	// Aggregate active assignments per user
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

	type teamRow struct {
		UserID                  string `gorm:"column:user_id"`
		ActiveAssignments       int    `gorm:"column:active_assignments"`
		AvgAssignmentAgeMinutes int    `gorm:"column:avg_assignment_age_minutes"`
		TotalExposureOwned      int64  `gorm:"column:total_exposure_owned"`
		EscalationCount         int    `gorm:"column:escalation_count"`
	}

	var rows []teamRow
	if err := s.db.WithContext(ctx).Raw(
		query,
		now, orgID,
	).Scan(&rows).Error; err != nil {
		return domain.TeamViewResponse{}, err
	}

	var (
		totalActiveAssignments int
		totalExposure          int64
		totalEscalationCount   int
		weightedAgeMinutes     int
		totalAssignmentsForAge int
	)

	members := make([]domain.TeamMemberWorkload, 0, len(rows))
	for _, row := range rows {
		// Update summary aggregates
		totalActiveAssignments += row.ActiveAssignments
		totalExposure += row.TotalExposureOwned
		totalEscalationCount += row.EscalationCount
		if row.ActiveAssignments > 0 {
			weightedAgeMinutes += row.AvgAssignmentAgeMinutes * row.ActiveAssignments
			totalAssignmentsForAge += row.ActiveAssignments
		}

		// Format average assignment age
		hours := row.AvgAssignmentAgeMinutes / 60
		minutes := row.AvgAssignmentAgeMinutes % 60
		var avgAge string
		if hours > 0 {
			avgAge = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			avgAge = fmt.Sprintf("%dm", minutes)
		}

		members = append(members, domain.TeamMemberWorkload{
			UserID:             row.UserID,
			ActiveAssignments:  row.ActiveAssignments,
			AvgAssignmentAge:   avgAge,
			TotalExposureOwned: row.TotalExposureOwned,
			EscalationCount:    row.EscalationCount,
		})
	}

	// Calculate global average age
	var globalAvgAge string
	if totalAssignmentsForAge > 0 {
		avgMins := weightedAgeMinutes / totalAssignmentsForAge
		h := avgMins / 60
		m := avgMins % 60
		if h > 0 {
			globalAvgAge = fmt.Sprintf("%dh %dm", h, m)
		} else {
			globalAvgAge = fmt.Sprintf("%dm", m)
		}
	} else {
		globalAvgAge = "0m"
	}

	return domain.TeamViewResponse{
		Members: members,
		Summary: domain.TeamSummary{
			TotalActiveAssignments: totalActiveAssignments,
			TotalExposure:          totalExposure,
			AvgAssignmentAge:       globalAvgAge,
			EscalationCount:        totalEscalationCount,
		},
		Currency: currency,
	}, nil
}

// GetInvoicePayments returns payment events associated with an invoice
func (s *Service) GetInvoicePayments(ctx context.Context, invoiceID string) (domain.InvoicePaymentsResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.InvoicePaymentsResponse{}, domain.ErrInvalidOrganization
	}

	// Query payment events linked to this invoice via metadata
	// Note: amount is not a column in payment_events, we must extract from payload or use what we can
	// But `payment_events` struct has `Provider`, `EventType`, etc.
	// We scan into a struct that captures payload
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

	type paymentRow struct {
		ProviderPaymentID string         `gorm:"column:provider_payment_id"`
		Provider          string         `gorm:"column:provider"`
		EventType         string         `gorm:"column:event_type"`
		ReceivedAt        time.Time      `gorm:"column:received_at"`
		Currency          string         `gorm:"column:currency"`
		Payload           datatypes.JSON `gorm:"column:payload"`
	}

	var rows []paymentRow
	if err := s.db.WithContext(ctx).Raw(query, orgID, invoiceID).Scan(&rows).Error; err != nil {
		return domain.InvoicePaymentsResponse{}, err
	}

	payments := make([]domain.PaymentDetail, 0, len(rows))
	for _, row := range rows {
		var payload map[string]interface{}
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			continue
		}

		// Extract amount (Stripe: data.object.amount)
		amount := int64(0)
		data, _ := payload["data"].(map[string]interface{})
		obj, _ := data["object"].(map[string]interface{})
		if val, ok := obj["amount"].(float64); ok {
			amount = int64(val)
		}

		// Status mapping
		status := "unknown"
		switch row.EventType {
		case "payment_succeeded", "charge.succeeded":
			status = "succeeded"
		case "payment_failed":
			status = "failed"
		case "refunded", "charge.refunded":
			status = "refunded"
		}

		// Extract card details (Stripe specific)
		// charges.data[0].payment_method_details.card or payment_method_details.card
		var method, brand, last4 string
		
		var paymentMethodDetails map[string]interface{}

		// Check top-level payment_method_details (Charge object)
		if pmd, ok := obj["payment_method_details"].(map[string]interface{}); ok {
			paymentMethodDetails = pmd
		} else if charges, ok := obj["charges"].(map[string]interface{}); ok {
			// Check inside charges (PaymentIntent object)
			if dataList, ok := charges["data"].([]interface{}); ok && len(dataList) > 0 {
				if firstCharge, ok := dataList[0].(map[string]interface{}); ok {
					if pmd, ok := firstCharge["payment_method_details"].(map[string]interface{}); ok {
						paymentMethodDetails = pmd
					}
				}
			}
		}

		if paymentMethodDetails != nil {
			if typeStr, ok := paymentMethodDetails["type"].(string); ok {
				method = typeStr
			}
			if card, ok := paymentMethodDetails["card"].(map[string]interface{}); ok {
				if b, ok := card["brand"].(string); ok {
					brand = b
				}
				if l, ok := card["last4"].(string); ok {
					last4 = l
				}
			}
		}

		payments = append(payments, domain.PaymentDetail{
			PaymentID:   row.ProviderPaymentID,
			Amount:      amount,
			Currency:    row.Currency,
			OccurredAt:  row.ReceivedAt.UTC(),
			Provider:    row.Provider,
			Method:      method,
			CardBrand:   brand,
			CardLast4:   last4,
			Status:      status,
		})
	}

	return domain.InvoicePaymentsResponse{
		Payments: payments,
	}, nil
}
func (s *Service) GetExposureAnalysis(ctx context.Context, req domain.ExposureAnalysisRequest) (domain.ExposureAnalysisResponse, error) {
	orgID, ok := orgcontext.OrgIDFromContext(ctx)
	if !ok || orgID == 0 {
		return domain.ExposureAnalysisResponse{}, domain.ErrInvalidOrganization
	}

	currency, err := s.loadOrgCurrency(ctx, orgID)
	if err != nil {
		return domain.ExposureAnalysisResponse{}, err
	}

	now := s.clock.Now().UTC()

	// 1. Calculate Total Exposure and Aging Buckets
	// We aggregate all finalized, non-voided, non-paid invoices
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

	var stats struct {
		TotalExposure int64
		CurrentAmount int64
		Bucket0To30   int64
		Bucket31To60  int64
		Bucket61To90  int64
		Bucket90Plus  int64
		OverdueCount  int
	}

	if err := s.db.WithContext(ctx).Raw(
		query,
		now,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency,
	).Scan(&stats).Error; err != nil {
		return domain.ExposureAnalysisResponse{}, err
	}

	// 2. Identify High Exposure Customers (Top 5)
	topCustomersQuery := `
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

	type topCustRow struct {
		EntityName  string `gorm:"column:entity_name"`
		AmountDue   int64  `gorm:"column:amount_due"`
		RiskScore   int    `gorm:"column:risk_score"`
		DaysOverdue int    `gorm:"column:days_overdue"`
	}

	var topRows []topCustRow
	if err := s.db.WithContext(ctx).Raw(
		topCustomersQuery,
		now,
		orgID, currency, string(ledgerdomain.SourceTypePayment), string(ledgerdomain.AccountCodeAccountsReceivable),
		orgID, currency,
	).Scan(&topRows).Error; err != nil {
		return domain.ExposureAnalysisResponse{}, err
	}

	topItems := make([]domain.InboxItem, 0, len(topRows))
	for _, r := range topRows {
		topItems = append(topItems, domain.InboxItem{
			EntityType:   "customer",
			EntityName:   r.EntityName,
			AmountDue:    r.AmountDue,
			RiskScore:    r.RiskScore,
			DaysOverdue:  r.DaysOverdue,
			Currency:     currency,
			RiskCategory: "high_exposure",
		})
	}

	// 3. Construct Response
	aging := []domain.ExposureBucket{
		{Bucket: "Current", Amount: stats.CurrentAmount, Count: 0},
		{Bucket: "1-30 Days", Amount: stats.Bucket0To30, Count: 0},
		{Bucket: "31-60 Days", Amount: stats.Bucket31To60, Count: 0},
		{Bucket: "61-90 Days", Amount: stats.Bucket61To90, Count: 0},
		{Bucket: "90+ Days", Amount: stats.Bucket90Plus, Count: 0},
	}

	// For Risk Category, we simplify:
	// "Overdue" -> Sum of all buckets > 0
	// "High Exposure" -> Sum of top 5 customers (simplified) or sum of all > 100k
	// "Failed Payment" -> We could query, but for now let's stick to Overdue vs Current
	riskCats := []domain.ExposureCategory{
		{Category: "Overdue", Amount: stats.Bucket0To30 + stats.Bucket31To60 + stats.Bucket61To90 + stats.Bucket90Plus, Count: stats.OverdueCount},
		{Category: "Current", Amount: stats.CurrentAmount, Count: 0}, // Count not easily available from agg
	}

	return domain.ExposureAnalysisResponse{
		TotalExposure:   stats.TotalExposure,
		Currency:        currency,
		ByRiskCategory:  riskCats,
		ByAgingBucket:   aging,
		TopHighExposure: topItems,
	}, nil
}

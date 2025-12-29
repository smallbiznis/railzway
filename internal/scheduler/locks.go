package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WorkSubscription struct {
	ID               snowflake.ID
	OrgID            snowflake.ID
	Status           subscriptiondomain.SubscriptionStatus
	ActivatedAt      *time.Time
	BillingCycleType string
}

type WorkBillingCycle struct {
	ID                 snowflake.ID
	OrgID              snowflake.ID
	SubscriptionID     snowflake.ID
	PeriodStart        time.Time
	PeriodEnd          time.Time
	Status             billingcycledomain.BillingCycleStatus
	ClosingStartedAt   *time.Time
	RatingCompletedAt  *time.Time
	InvoicedAt         *time.Time
	InvoiceFinalizedAt *time.Time
	ClosedAt           *time.Time
}

func (s *Scheduler) FetchSubscriptionsForWork(ctx context.Context, status subscriptiondomain.SubscriptionStatus, limit int) ([]WorkSubscription, error) {
	var subscriptions []WorkSubscription
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		subscriptions, err = s.fetchSubscriptionsForWork(ctx, tx, status, limit)
		return err
	})
	if err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (s *Scheduler) FetchBillingCyclesForWork(ctx context.Context, status billingcycledomain.BillingCycleStatus, limit int) ([]WorkBillingCycle, error) {
	return s.fetchBillingCyclesForWork(ctx, `status = ?`, []any{status}, limit)
}

func (s *Scheduler) fetchSubscriptionsForWork(ctx context.Context, tx *gorm.DB, status subscriptiondomain.SubscriptionStatus, limit int) ([]WorkSubscription, error) {
	var subscriptions []WorkSubscription
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, status, activated_at, billing_cycle_type
		 FROM subscriptions
		 WHERE status = ?
		 ORDER BY id
		 FOR UPDATE SKIP LOCKED
		 LIMIT ?`,
		status,
		limit,
	).Scan(&subscriptions).Error
	if err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (s *Scheduler) fetchBillingCyclesForWork(ctx context.Context, where string, args []any, limit int) ([]WorkBillingCycle, error) {
	if limit <= 0 {
		limit = s.cfg.BatchSize
	}
	var cycles []WorkBillingCycle
	query := fmt.Sprintf(
		`SELECT id, org_id, subscription_id, period_start, period_end, status,
		        closing_started_at, rating_completed_at, invoiced_at,
		        invoice_finalized_at, closed_at
		 FROM billing_cycles
		 WHERE %s
		 ORDER BY period_end ASC, id ASC
		 FOR UPDATE SKIP LOCKED
		 LIMIT ?`,
		where,
	)
	args = append(args, limit)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.WithContext(ctx).Raw(query, args...).Scan(&cycles).Error
	})
	if err != nil {
		return nil, err
	}
	return cycles, nil
}

func (s *Scheduler) findOpenCycle(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (*WorkBillingCycle, int64, error) {
	var count int64
	if err := tx.WithContext(ctx).Raw(
		`SELECT COUNT(1)
		 FROM billing_cycles
		 WHERE org_id = ? AND subscription_id = ? AND status = ?`,
		orgID,
		subscriptionID,
		billingcycledomain.BillingCycleStatusOpen,
	).Scan(&count).Error; err != nil {
		return nil, 0, err
	}

	var cycle WorkBillingCycle
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, period_start, period_end, status,
		        closing_started_at, rating_completed_at, invoiced_at,
		        invoice_finalized_at, closed_at
		 FROM billing_cycles
		 WHERE org_id = ? AND subscription_id = ? AND status = ?
		 ORDER BY period_end DESC
		 LIMIT 1
		 FOR UPDATE`,
		orgID,
		subscriptionID,
		billingcycledomain.BillingCycleStatusOpen,
	).Scan(&cycle).Error
	if err != nil {
		return nil, 0, err
	}
	if cycle.ID == 0 {
		return nil, count, nil
	}
	return &cycle, count, nil
}

func (s *Scheduler) findLastCycle(ctx context.Context, tx *gorm.DB, orgID, subscriptionID snowflake.ID) (*WorkBillingCycle, error) {
	var cycle WorkBillingCycle
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, period_start, period_end, status,
		        closing_started_at, rating_completed_at, invoiced_at,
		        invoice_finalized_at, closed_at
		 FROM billing_cycles
		 WHERE org_id = ? AND subscription_id = ?
		 ORDER BY period_end DESC
		 LIMIT 1`,
		orgID,
		subscriptionID,
	).Scan(&cycle).Error
	if err != nil {
		return nil, err
	}
	if cycle.ID == 0 {
		return nil, nil
	}
	return &cycle, nil
}

func (s *Scheduler) insertCycle(ctx context.Context, tx *gorm.DB, cycleID, orgID, subscriptionID snowflake.ID, periodStart, periodEnd, now time.Time) error {
	openedAt := now
	return tx.WithContext(ctx).Exec(
		`INSERT INTO billing_cycles (
			id, org_id, subscription_id, period_start, period_end, status,
			opened_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cycleID,
		orgID,
		subscriptionID,
		periodStart,
		periodEnd,
		billingcycledomain.BillingCycleStatusOpen,
		openedAt,
		now,
		now,
	).Error
}

func (s *Scheduler) lockCycleForUpdate(ctx context.Context, tx *gorm.DB, cycleID snowflake.ID) (*WorkBillingCycle, error) {
	var cycle WorkBillingCycle
	err := tx.WithContext(ctx).Raw(
		`SELECT id, org_id, subscription_id, period_start, period_end, status,
		        closing_started_at, rating_completed_at, invoiced_at,
		        invoice_finalized_at, closed_at
		 FROM billing_cycles
		 WHERE id = ?
		 FOR UPDATE`,
		cycleID,
	).Scan(&cycle).Error
	if err != nil {
		return nil, err
	}
	if cycle.ID == 0 {
		return nil, nil
	}
	return &cycle, nil
}

func (s *Scheduler) markCycleClosing(ctx context.Context, cycleID snowflake.ID, now time.Time) (bool, error) {
	updated := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := s.lockCycleForUpdate(ctx, tx, cycleID)
		if err != nil {
			return err
		}
		if cycle == nil || cycle.Status != billingcycledomain.BillingCycleStatusOpen {
			return nil
		}
		if now.Before(cycle.PeriodEnd) {
			return nil
		}
		updated, err = s.markCycleClosingTx(ctx, tx, cycleID, now)
		return err
	})
	return updated, err
}

func (s *Scheduler) markCycleClosingTx(ctx context.Context, tx *gorm.DB, cycleID snowflake.ID, now time.Time) (bool, error) {
	result := tx.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET status = ?, closing_started_at = COALESCE(closing_started_at, ?), updated_at = ?
		 WHERE id = ? AND status = ?`,
		billingcycledomain.BillingCycleStatusClosing,
		now,
		now,
		cycleID,
		billingcycledomain.BillingCycleStatusOpen,
	)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *Scheduler) markRatingCompleted(ctx context.Context, cycleID snowflake.ID, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := s.lockCycleForUpdate(ctx, tx, cycleID)
		if err != nil {
			return err
		}
		if cycle == nil || cycle.Status != billingcycledomain.BillingCycleStatusClosing {
			return nil
		}
		return tx.WithContext(ctx).Exec(
			`UPDATE billing_cycles
			 SET rating_completed_at = COALESCE(rating_completed_at, ?),
			     last_error = NULL,
			     last_error_at = NULL,
			     updated_at = ?
			 WHERE id = ? AND status = ?`,
			now,
			now,
			cycleID,
			billingcycledomain.BillingCycleStatusClosing,
		).Error
	})
}

func (s *Scheduler) markCycleClosed(ctx context.Context, cycleID snowflake.ID, now time.Time) (bool, error) {
	updated := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := s.lockCycleForUpdate(ctx, tx, cycleID)
		if err != nil {
			return err
		}
		if cycle == nil || cycle.Status != billingcycledomain.BillingCycleStatusClosing {
			return nil
		}
		if cycle.RatingCompletedAt == nil {
			return invoicedomain.ErrMissingRatingResults
		}
		result := tx.WithContext(ctx).Exec(
			`UPDATE billing_cycles
			 SET status = ?, closed_at = COALESCE(closed_at, ?),
			     last_error = NULL,
			     last_error_at = NULL,
			     updated_at = ?
			 WHERE id = ? AND status = ? AND rating_completed_at IS NOT NULL`,
			billingcycledomain.BillingCycleStatusClosed,
			now,
			now,
			cycleID,
			billingcycledomain.BillingCycleStatusClosing,
		)
		if result.Error != nil {
			return result.Error
		}
		updated = result.RowsAffected > 0
		return nil
	})
	return updated, err
}

func (s *Scheduler) markCycleInvoiced(ctx context.Context, cycleID snowflake.ID, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := s.lockCycleForUpdate(ctx, tx, cycleID)
		if err != nil {
			return err
		}
		if cycle == nil || cycle.Status != billingcycledomain.BillingCycleStatusClosed {
			return nil
		}
		return tx.WithContext(ctx).Exec(
			`UPDATE billing_cycles
			 SET invoiced_at = COALESCE(invoiced_at, ?),
			     last_error = NULL,
			     last_error_at = NULL,
			     updated_at = ?
			 WHERE id = ? AND status = ?`,
			now,
			now,
			cycleID,
			billingcycledomain.BillingCycleStatusClosed,
		).Error
	})
}

func (s *Scheduler) markCycleInvoiceFinalized(ctx context.Context, cycleID snowflake.ID, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cycle, err := s.lockCycleForUpdate(ctx, tx, cycleID)
		if err != nil {
			return err
		}
		if cycle == nil || cycle.Status != billingcycledomain.BillingCycleStatusClosed {
			return nil
		}
		return tx.WithContext(ctx).Exec(
			`UPDATE billing_cycles
			 SET invoice_finalized_at = COALESCE(invoice_finalized_at, ?),
			     updated_at = ?
			 WHERE id = ? AND status = ?`,
			now,
			now,
			cycleID,
			billingcycledomain.BillingCycleStatusClosed,
		).Error
	})
}

func (s *Scheduler) recordCycleError(ctx context.Context, cycleID snowflake.ID, err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	now := time.Now().UTC()
	if updateErr := s.db.WithContext(ctx).Exec(
		`UPDATE billing_cycles
		 SET last_error = ?, last_error_at = ?, updated_at = ?
		 WHERE id = ?`,
		message,
		now,
		now,
		cycleID,
	).Error; updateErr != nil {
		s.log.Warn("failed to record cycle error", zap.String("cycle_id", cycleID.String()), zap.Error(updateErr))
		return updateErr
	}
	return nil
}

func (s *Scheduler) hasRatingResults(ctx context.Context, cycleID snowflake.ID) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT COUNT(1)
		 FROM rating_results
		 WHERE billing_cycle_id = ?`,
		cycleID,
	).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Scheduler) canEndSubscription(ctx context.Context, orgID, subscriptionID snowflake.ID) (bool, error) {
	var openCount int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT COUNT(1)
		 FROM billing_cycles
		 WHERE org_id = ? AND subscription_id = ? AND status IN (?, ?)`,
		orgID,
		subscriptionID,
		billingcycledomain.BillingCycleStatusOpen,
		billingcycledomain.BillingCycleStatusClosing,
	).Scan(&openCount).Error; err != nil {
		return false, err
	}
	if openCount > 0 {
		return false, nil
	}

	var invoiceCount int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT COUNT(1)
		 FROM billing_cycles bc
		 LEFT JOIN invoices i ON i.billing_cycle_id = bc.id
		 WHERE bc.org_id = ? AND bc.subscription_id = ? AND bc.status = ?
		   AND (i.id IS NULL OR i.status NOT IN (?, ?))`,
		orgID,
		subscriptionID,
		billingcycledomain.BillingCycleStatusClosed,
		invoicedomain.InvoiceStatusFinalized,
		invoicedomain.InvoiceStatusVoid,
	).Scan(&invoiceCount).Error; err != nil {
		return false, err
	}
	if invoiceCount > 0 {
		return false, nil
	}

	return true, nil
}

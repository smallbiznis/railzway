package scheduler

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	billingcycledomain "github.com/smallbiznis/valora/internal/billingcycle/domain"
	"gorm.io/gorm"
)

func (s *Scheduler) upsertBillingCycleStats(ctx context.Context, dbConn *gorm.DB, cycleID, orgID snowflake.ID, periodStart time.Time, status billingcycledomain.BillingCycleStatus, now time.Time) error {
	if dbConn == nil {
		dbConn = s.db
	}
	return dbConn.WithContext(ctx).Exec(
		`INSERT INTO billing_cycle_stats (billing_cycle_id, org_id, period_start, status, total_revenue, invoice_count, updated_at)
		 VALUES (?, ?, ?, ?, 0, 0, ?)
		 ON CONFLICT (billing_cycle_id)
		 DO UPDATE SET period_start = EXCLUDED.period_start,
		               status = EXCLUDED.status,
		               updated_at = EXCLUDED.updated_at`,
		cycleID,
		orgID,
		periodStart,
		string(status),
		now,
	).Error
}
